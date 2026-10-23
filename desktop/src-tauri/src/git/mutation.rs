use super::{git, request_string};
use crate::error::{HostError, HostResult};
use serde_json::{json, Value};
use std::{collections::HashMap, io::Write, path::Path, sync::{Arc, Mutex, OnceLock, Weak}};

fn failure(message: impl Into<String>) -> HostError { HostError::new("GIT_MUTATION",message) }
fn head(root:&Path)->HostResult<String> {Ok(git(root,&["rev-parse","--verify","HEAD"])?.trim().into())}
fn dirty(root:&Path)->HostResult<bool> {Ok(!git(root,&["status","--porcelain","--untracked-files=all","--","."])?.trim().is_empty())}

fn guard(root:&Path,sha:&str,expected:&str)->HostResult<String> {
    for value in [sha,expected] {
        if ![40,64].contains(&value.len()) || !value.bytes().all(|byte|byte.is_ascii_hexdigit()) {return Err(failure("Некорректный идентификатор коммита"));}
    }
    let resolved=git(root,&["rev-parse","--verify",&format!("{sha}^{{commit}}")])?;
    if !resolved.trim().eq_ignore_ascii_case(sha) {return Err(failure("Коммит не найден"));}
    if git(root,&["for-each-ref",&format!("--contains={sha}"),"--format=%(refname)"])?.trim().is_empty() {
        git(root,&["merge-base","--is-ancestor",sha,"HEAD"]).map_err(|_|failure("Коммит не принадлежит истории репозитория"))?;
    }
    let previous=head(root)?;
    if previous!=expected {return Err(failure("История изменилась. Обновите список коммитов"));}
    git(root,&["symbolic-ref","--quiet","HEAD"]).map_err(|_|failure("Переключитесь на ветку перед восстановлением"))?;
    for marker in ["MERGE_HEAD","REBASE_HEAD","REVERT_HEAD","CHERRY_PICK_HEAD"] {
        if git(root,&["rev-parse","--verify","--quiet",marker]).is_ok() {return Err(failure("Сначала завершите текущую операцию Git"));}
    }
    for marker in ["rebase-merge","rebase-apply"] {
        let path=git(root,&["rev-parse","--git-path",marker])?;
        let path=Path::new(path.trim());
        if (if path.is_absolute(){path.to_path_buf()}else{root.join(path)}).exists() {return Err(failure("Сначала завершите текущую операцию Git"));}
    }
    Ok(previous)
}

fn ignored_guard(root:&Path,sha:&str,changed:Option<&str>)->HostResult<()> {
    let ignored=git(root,&["ls-files","--others","--ignored","--exclude-standard","--directory","-z","--","."])?;
    let paths=match changed {Some(paths)=>paths.to_owned(),None=>git(root,&["ls-tree","-rz","--name-only",sha,"--","."])?};
    let ignore_case=git(root,&["config","--get","core.ignorecase"]).unwrap_or_default().trim()=="true";
    let normalize=|value:&str| {let value=value.trim_end_matches('/');if ignore_case{value.to_lowercase()}else{value.to_owned()}};
    let paths=paths.split('\0').filter(|path|!path.is_empty()).map(normalize).collect::<Vec<_>>();
    for ignored in ignored.split('\0').filter(|path|!path.is_empty()) {
        let path=normalize(ignored);
        if paths.iter().any(|tracked|tracked==&path || tracked.starts_with(&format!("{path}/")) || path.starts_with(&format!("{tracked}/"))) {
            return Err(failure(format!("Восстановление может заменить игнорируемый файл или каталог «{ignored}». Переместите его в безопасное место")));
        }
    }
    Ok(())
}

fn backup(root:&Path,sha:&str)->HostResult<Option<String>> {
    if !dirty(root)? {return Ok(None);}
    git(root,&["stash","push","--include-untracked","--message",&format!("Prime restore backup before {}",&sha[..12]),"--","."])?;
    let backup=git(root,&["rev-parse","--verify","refs/stash"])?.trim().to_owned();
    if dirty(root)? {return Err(failure(format!("Локальные изменения сохранены в stash {backup}, но рабочее дерево осталось изменённым")));}
    Ok(Some(backup))
}

fn recover(root:&Path,previous:&str,backup:&Option<String>,changed:bool,error:HostError)->HostError {
    let note=backup.as_ref().map(|backup|format!("\nЛокальные изменения сохранены в stash {backup}. Для восстановления: git stash apply --index {backup}")).unwrap_or_default();
    if head(root).unwrap_or_default()!=previous {return failure(format!("{}\nHEAD изменился во время восстановления. Автоматический откат отменён; проверьте git status.{note}",error.message));}
    if changed {
        if let Err(restore)=git(root,&["restore",&format!("--source={previous}"),"--staged","--worktree","--","."]) {return failure(format!("{}\nНе удалось восстановить рабочее дерево: {}{note}",error.message,restore.message));}
    }
    if let Some(backup)=backup {
        if let Err(restore)=git(root,&["stash","apply","--index",backup]) {return failure(format!("{}\nНе удалось применить резервную копию: {}{note}",error.message,restore.message));}
    }
    error
}

pub fn apply(root:&Path,request:&Value,revert:bool)->HostResult<Value> {
    static LOCKS:OnceLock<Mutex<HashMap<String,Weak<Mutex<()>>>>>=OnceLock::new();
    let identity=git(root,&["rev-parse","--absolute-git-dir"])?;
    let gate={let mut gates=LOCKS.get_or_init(Default::default).lock().unwrap();gates.retain(|_,gate|gate.strong_count()>0);match gates.get(&identity).and_then(Weak::upgrade){Some(gate)=>gate,None=>{let gate=Arc::new(Mutex::new(()));gates.insert(identity,Arc::downgrade(&gate));gate}}};
    let _guard=gate.lock().unwrap();
    let sha=request_string(request,"sha")?;
    let previous=guard(root,sha,request_string(request,"expectedHead")?)?;
    let mut patch=None;
    let mut changed_paths=None;
    if revert {
        let parents=git(root,&["show","--no-patch","--format=%P",sha])?;
        let parent=parents.split_whitespace().next();
        let output=match parent {
            Some(parent)=>git(root,&["diff","--no-renames","--binary","--full-index","--no-ext-diff","--no-textconv",sha,parent,"--","."])?,
            None=>git(root,&["diff-tree","--no-renames","--root","--no-commit-id","-p","--binary","--full-index","--no-ext-diff","--no-textconv","-R",sha,"--","."])?
        };
        if output.trim().is_empty(){return Ok(json!({"previousHead":previous,"currentHead":previous,"restoredCommit":sha,"changed":false}));}
        if output.len()>=8*1024*1024 {return Err(failure("Патч превышает безопасный размер предпросмотра"));}
        changed_paths=Some(match parent {Some(parent)=>git(root,&["diff","--no-renames","--name-only","--relative","-z",sha,parent,"--","."])?,None=>git(root,&["diff-tree","--no-renames","--root","--no-commit-id","--name-only","--relative","-r","-z",sha,"--","."])?});
        let mut file=tempfile::NamedTempFile::new()?;file.write_all(output.as_bytes())?;file.flush()?;patch=Some(file);
    }
    ignored_guard(root,sha,changed_paths.as_deref())?;
    let backup=backup(root,sha)?;
    let mut changed=false;
    let result=(|| {
        if head(root)?!=previous {return Err(failure("История изменилась. Обновите список коммитов"));}
        if dirty(root)? {return Err(failure("Появились новые локальные изменения. Повторите операцию"));}
        if let Some(patch)=&patch {changed=true;git(root,&["apply","--3way","--index","--binary",&patch.path().to_string_lossy()])?;}
        else if !git(root,&["diff","--name-only",&previous,sha,"--","."])?.trim().is_empty() {changed=true;git(root,&["restore",&format!("--source={sha}"),"--staged","--worktree","--","."])?;}
        if head(root)?!=previous {return Err(failure("История изменилась. Проверьте git status"));}
        let staged=!git(root,&["diff","--no-renames","--cached","--name-only","--","."])?.trim().is_empty();
        if staged {git(root,&["commit","--only","-m",&format!("{} {}",if revert{"Revert project changes from"}else{"Restore project to"},&sha[..12]),"--","."])?;}
        Ok(json!({"previousHead":previous,"currentHead":head(root)?,"restoredCommit":sha,"changed":staged,"backupStash":backup}))
    })();
    result.map_err(|error|recover(root,&previous,&backup,changed,error))
}

#[cfg(test)]
mod tests {
    use super::*;
    fn fixture()->(tempfile::TempDir,String,String) {
        let directory=tempfile::tempdir().unwrap();let root=directory.path();
        git(root,&["init","-q"]).unwrap();git(root,&["config","core.autocrlf","false"]).unwrap();git(root,&["config","user.name","Fixture"]).unwrap();git(root,&["config","user.email","fixture@example.test"]).unwrap();
        std::fs::write(root.join("file.txt"),"first\n").unwrap();git(root,&["add","."]).unwrap();git(root,&["commit","-qm","First"]).unwrap();let first=head(root).unwrap();
        std::fs::write(root.join("file.txt"),"second\n").unwrap();git(root,&["commit","-qam","Second"]).unwrap();let second=head(root).unwrap();
        (directory,first,second)
    }
    #[test]
    fn restore_preserves_staged_and_untracked_work_in_backup() {
        let (directory,first,second)=fixture();let root=directory.path();
        std::fs::write(root.join("file.txt"),"local work\n").unwrap();git(root,&["add","file.txt"]).unwrap();std::fs::write(root.join("notes.txt"),"private notes\n").unwrap();
        let result=apply(root,&json!({"sha":first,"expectedHead":second}),false).unwrap();
        assert_eq!(std::fs::read_to_string(root.join("file.txt")).unwrap(),"first\n");assert_eq!(result["changed"],true);assert_ne!(head(root).unwrap(),second);
        let backup=result["backupStash"].as_str().unwrap();let saved=git(root,&["stash","show","--include-untracked","-p",backup]).unwrap();assert!(saved.contains("local work"));assert!(saved.contains("private notes"));
    }
    #[test]
    fn revert_selected_commit_preserves_later_commits() {
        let (directory,_,second)=fixture();let root=directory.path();
        std::fs::write(root.join("later.txt"),"later work\n").unwrap();git(root,&["add","."]).unwrap();git(root,&["commit","-qm","Later"]).unwrap();let before=head(root).unwrap();
        let result=apply(root,&json!({"sha":second,"expectedHead":before}),true).unwrap();assert_eq!(result["changed"],true);
        assert_eq!(std::fs::read_to_string(root.join("file.txt")).unwrap(),"first\n");assert_eq!(std::fs::read_to_string(root.join("later.txt")).unwrap(),"later work\n");
        assert!(git(root,&["merge-base","--is-ancestor",&before,"HEAD"]).is_ok());
    }
    #[test]
    fn conflict_recovers_head_index_and_local_backup() {
        let (directory,_,second)=fixture();let root=directory.path();
        std::fs::write(root.join("file.txt"),"conflicting third version\n").unwrap();git(root,&["commit","-qam","Third"]).unwrap();let before=head(root).unwrap();std::fs::write(root.join("notes.txt"),"working notes\n").unwrap();
        assert!(apply(root,&json!({"sha":second,"expectedHead":before}),true).is_err());assert_eq!(head(root).unwrap(),before);
        assert_eq!(std::fs::read_to_string(root.join("file.txt")).unwrap(),"conflicting third version\n");assert_eq!(std::fs::read_to_string(root.join("notes.txt")).unwrap(),"working notes\n");assert!(git(root,&["ls-files","--unmerged"]).unwrap().trim().is_empty());
    }
    #[test]
    fn ignored_collision_is_rejected_without_changing_files() {
        let (directory,_,_)=fixture();let root=directory.path();
        std::fs::write(root.join("secret.txt"),"old tracked\n").unwrap();git(root,&["add","."]).unwrap();git(root,&["commit","-qm","Tracked secret"]).unwrap();let target=head(root).unwrap();
        git(root,&["rm","secret.txt"]).unwrap();std::fs::write(root.join(".gitignore"),"secret.txt\n").unwrap();git(root,&["add","."]).unwrap();git(root,&["commit","-qm","Ignore secret"]).unwrap();let before=head(root).unwrap();std::fs::write(root.join("secret.txt"),"local ignored\n").unwrap();
        assert!(apply(root,&json!({"sha":target,"expectedHead":before}),false).is_err());assert_eq!(head(root).unwrap(),before);assert_eq!(std::fs::read_to_string(root.join("secret.txt")).unwrap(),"local ignored\n");
    }
}
