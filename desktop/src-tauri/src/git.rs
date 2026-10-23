use crate::error::{HostError, HostResult};
use serde_json::{json, Value};
use std::{io::Read, path::Path, process::{Command, Stdio}, thread, time::{Duration, Instant}};
mod mutation;

fn capture(mut input: impl Read) -> std::io::Result<Vec<u8>> {
    let mut retained = Vec::new();
    let mut buffer = [0u8; 8192];
    loop {
        let count = input.read(&mut buffer)?;
        if count == 0 { return Ok(retained); }
        let keep = count.min((8 * 1024 * 1024usize).saturating_sub(retained.len()));
        retained.extend_from_slice(&buffer[..keep]);
    }
}

fn run(root: &Path, program: &str, args: &[&str]) -> HostResult<String> {
    let mut command = Command::new(program);
    command.args(args).current_dir(root).env("GIT_TERMINAL_PROMPT", "0").env("GH_PROMPT_DISABLED", "1")
        .stdin(Stdio::null()).stdout(Stdio::piped()).stderr(Stdio::piped());
    #[cfg(windows)] {
        use std::os::windows::process::CommandExt;
        command.creation_flags(0x08000000);
    }
    let mut child = command.spawn()?;
    let stdout = child.stdout.take().unwrap();
    let stderr = child.stderr.take().unwrap();
    let out = thread::spawn(move || capture(stdout));
    let err = thread::spawn(move || capture(stderr));
    let start = Instant::now();
    let status = loop {
        if let Some(status) = child.try_wait()? { break status; }
        if start.elapsed() > Duration::from_secs(90) {
            let _ = child.kill();
            let _ = child.wait();
            return Err(HostError::new("COMMAND_TIMEOUT", format!("{program} timed out")));
        }
        thread::sleep(Duration::from_millis(20));
    };
    let output = out.join().map_err(|_| HostError::new("COMMAND_OUTPUT", "Output reader failed"))??;
    let errors = err.join().map_err(|_| HostError::new("COMMAND_OUTPUT", "Error reader failed"))??;
    let check_result = program == "gh" && args.starts_with(&["pr", "checks"])
        && matches!(status.code(), Some(1 | 8)) && !output.is_empty();
    if !status.success() && !check_result {
        return Err(HostError::new("COMMAND_FAILED", String::from_utf8_lossy(&errors).trim().to_owned()));
    }
    Ok(String::from_utf8_lossy(&output).into_owned())
}

fn git(root: &Path, args: &[&str]) -> HostResult<String> { run(root, "git", args) }
fn gh(root: &Path, args: &[&str]) -> HostResult<Value> { Ok(serde_json::from_str(&run(root, "gh", args)?)?) }

fn commit(record: &str) -> Value {
    let fields: Vec<_> = record.trim().split('\0').collect();
    let value = |index| fields.get(index).copied().unwrap_or("");
    json!({"sha":value(0),"parents":value(1).split_whitespace().collect::<Vec<_>>(),"title":value(2),"author":value(3),"date":value(4),"refs":value(5)})
}

fn changes(root: &Path) -> HostResult<Value> {
    let prefix = git(root, &["rev-parse", "--show-prefix"])?;
    let prefix = prefix.trim();
    let output = git(root, &["status", "--porcelain=v1", "-z", "--untracked-files=all", "--", "."])?;
    let mut records = output.split('\0');
    let mut result = Vec::new();
    while let Some(record) = records.next() {
        if record.len() < 3 || !record.is_char_boundary(3) { continue; }
        let status = &record[..2];
        let path = &record[3..];
        let original = if status.contains('R') || status.contains('C') { records.next() } else { None };
        if let Some(relative) = path.strip_prefix(prefix) {
            result.push(json!({"path":relative,"status":status,"originalPath":original.and_then(|path|path.strip_prefix(prefix))}));
        }
    }
    Ok(json!(result))
}

fn request_string<'a>(request: &'a Value, key: &str) -> HostResult<&'a str> {
    request[key].as_str().ok_or_else(|| HostError::new("INVALID_ARGUMENT", format!("Missing {key}")))
}

fn number(request:&Value)->HostResult<String> {
    request["number"].as_u64().filter(|value|*value>0).map(|value|value.to_string()).ok_or_else(||HostError::new("INVALID_ARGUMENT","Invalid issue number"))
}

fn body_file(request:&Value)->HostResult<tempfile::NamedTempFile> {
    use std::io::Write;
    let body=request_string(request,"body")?;
    if body.len()>60000 {return Err(HostError::new("INVALID_ARGUMENT","Body exceeds 60000 bytes"));}
    let mut file=tempfile::NamedTempFile::new()?;
    file.write_all(body.as_bytes())?;
    file.flush()?;
    Ok(file)
}

fn read_text(path: &Path) -> HostResult<String> {
    let bytes = match std::fs::metadata(path) {
        Ok(metadata) if metadata.is_file() && metadata.len() <= 2 * 1024 * 1024 => std::fs::read(path)?,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Vec::new(),
        _ => return Err(HostError::new("INVALID_FILE", "Diff requires a file no larger than 2 MiB")),
    };
    if bytes.contains(&0) {
        return Err(HostError::new("BINARY_FILE", "Binary files cannot be previewed"));
    }
    String::from_utf8(bytes).map_err(|_| HostError::new("INVALID_ENCODING", "File is not UTF-8"))
}

pub fn session_diff(root: &Path, relative: &str, history: &Value) -> HostResult<Value> {
    let after = read_text(&crate::files::scoped_path(root, relative, true)?)?;
    let wanted = relative.replace('\\', "/");
    let mut versions = history
        .as_array()
        .map(|files| {
            files
                .iter()
                .filter(|file| {
                    file["path"].as_str().is_some_and(|path| {
                        let path = path.replace('\\', "/");
                        path == wanted || path.ends_with(&format!("/{wanted}"))
                    })
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    versions.sort_by_key(|file| file["version"].as_i64().unwrap_or(0));
    let before = versions
        .first()
        .and_then(|file| file["content"].as_str())
        .ok_or_else(|| HostError::new("NO_SESSION_VERSION", "This file has no version in the session history"))?
        .to_string();
    let patch = similar::TextDiff::from_lines(&before, &after)
        .unified_diff()
        .header(relative, relative)
        .to_string();
    Ok(json!({"path":relative,"source":"session","before":before,"after":after,"patch":patch}))
}

pub fn dispatch(root: &Path, method: &str, args: &[Value]) -> HostResult<Value> {
    if method == "changes" { return changes(root); }
    if method == "diff" {
        if args.get(3).and_then(Value::as_str) != Some("git") {
            return Err(HostError::unsupported("diff.session"));
        }
        let relative=args.get(2).and_then(Value::as_str).ok_or_else(||HostError::new("INVALID_ARGUMENT","Expected file path"))?;
        let absolute=crate::files::scoped_path(root,relative,true)?;
        let bytes=match std::fs::metadata(&absolute) {
            Ok(metadata) if metadata.is_file() && metadata.len()<=2*1024*1024=>std::fs::read(&absolute)?,
            Err(error) if error.kind()==std::io::ErrorKind::NotFound=>Vec::new(),
            _=>return Err(HostError::new("INVALID_FILE","Diff requires a file no larger than 2 MiB"))
        };
        if bytes.contains(&0) {return Err(HostError::new("BINARY_FILE","Binary files cannot be previewed"));}
        let after=String::from_utf8(bytes).map_err(|_|HostError::new("INVALID_ENCODING","File is not UTF-8"))?;
        let status=changes(root)?;
        let change=status.as_array().unwrap().iter().find(|change|change["path"].as_str()==Some(relative));
        let original=change.and_then(|change|change["originalPath"].as_str()).unwrap_or(relative);
        let is_new=change.and_then(|change|change["status"].as_str()).map(|value|value=="??" || value.contains('A')).unwrap_or(false);
        let before=if is_new {String::new()}else{git(root,&["show",&format!("HEAD:./{}",original.replace('\\',"/"))])?};
        if before.contains('\0') {return Err(HostError::new("BINARY_FILE","Binary files cannot be previewed"));}
        let patch=similar::TextDiff::from_lines(&before,&after).unified_diff().header(original,relative).to_string();
        return Ok(json!({"path":relative,"source":"git","before":before,"after":after,"status":change.map(|change|&change["status"]),"originalPath":change.map(|change|&change["originalPath"]),"patch":patch}));
    }
    let request = args.get(1).unwrap_or(&Value::Null);
    let action = request_string(request, "action")?;
    match action {
        "restoreCommit" | "revertCommit" => mutation::apply(root,request,action=="revertCommit"),
        "history" => {
            let head = git(root, &["rev-parse", "HEAD"])?;
            let branch = git(root, &["rev-parse", "--abbrev-ref", "HEAD"])?;
            let status = git(root, &["status", "--porcelain", "--untracked-files=all"])?;
            let output = git(root, &["log", "--all", "HEAD", "--date-order", "--max-count=100", "--format=%H%x00%P%x00%s%x00%an%x00%aI%x00%D%x1e", "--", "."])?;
            Ok(json!({"currentHead":head.trim(),"branch":branch.trim(),"clean":status.trim().is_empty(),"commits":output.split('\x1e').filter(|record|!record.trim().is_empty()).map(commit).collect::<Vec<_>>()}))
        },
        "showCommit" => {
            let sha = request_string(request, "sha")?;
            if ![40,64].contains(&sha.len()) || !sha.bytes().all(|byte|byte.is_ascii_hexdigit()) {return Err(HostError::new("INVALID_COMMIT", "Expected a full commit hash"));}
            git(root, &["cat-file", "-e", &format!("{sha}^{{commit}}")])?;
            let head = git(root, &["rev-parse", "HEAD"])?;
            let metadata = git(root, &["show", "--no-patch", "--format=%H%x00%P%x00%s%x00%an%x00%aI%x00%D", sha])?;
            let output = git(root, &["show", "--format=", "--no-ext-diff", "--no-textconv", "--stat", "--patch", sha, "--", "."])?;
            let truncated = output.chars().count() > 300000;
            Ok(json!({"currentHead":head.trim(),"commit":commit(&metadata),"diff":output.chars().take(300000).collect::<String>(),"truncated":truncated}))
        },
        "branches" => Ok(json!(git(root, &["for-each-ref", "--format=%(refname:short)", "refs/heads"])?.lines().filter(|line|!line.trim().is_empty()).collect::<Vec<_>>())),
        "status" => {
            if run(root,"gh",&["--version"]).is_err() { return Ok(json!({"available":false,"authenticated":false,"error":"Установите GitHub CLI"})); }
            let user = match gh(root,&["api","user"]) {Ok(value)=>value,Err(_)=>return Ok(json!({"available":true,"authenticated":false,"error":"Выполните gh auth login"}))};
            let branch = git(root,&["rev-parse","--abbrev-ref","HEAD"]).unwrap_or_default();
            match gh(root,&["repo","view","--json","nameWithOwner,url,defaultBranchRef"]) {
                Ok(repository)=>Ok(json!({"available":true,"authenticated":true,"account":user["login"],"branch":branch.trim(),"repository":repository["nameWithOwner"],"url":repository["url"],"defaultBranch":repository["defaultBranchRef"]["name"]})),
                Err(_)=>Ok(json!({"available":true,"authenticated":true,"account":user["login"],"branch":branch.trim(),"error":"Проект не связан с репозиторием GitHub"}))
            }
        },
        "issues" | "pulls" => {
            let state = request["state"].as_str().unwrap_or("open");
            if !["open","closed","all"].contains(&state) {return Err(HostError::new("INVALID_ARGUMENT","Invalid list state"));}
            let limit = request["limit"].as_u64().unwrap_or(50).clamp(1,1000).to_string();
            let fields = if action=="pulls" {"number,title,state,url,updatedAt,author,labels,headRefName,baseRefName,isDraft,reviewDecision"} else {"number,title,state,url,updatedAt,author,labels"};
            gh(root,&[if action=="pulls" {"pr"}else{"issue"},"list","--state",state,"--limit",&limit,"--json",fields])
        },
        "issue" | "pull" | "diff" => {
            let number = request["number"].as_u64().filter(|number|*number>0).ok_or_else(||HostError::new("INVALID_ARGUMENT","Invalid issue number"))?.to_string();
            if action=="diff" {return Ok(json!(run(root,"gh",&["pr","diff",&number])?.chars().take(400000).collect::<String>()));}
            let mut detail=gh(root,&[if action=="pull" {"pr"}else{"issue"},"view",&number,"--json",if action=="pull" {"number,title,state,url,updatedAt,author,labels,headRefName,baseRefName,isDraft,reviewDecision,body,comments,reviews,files"}else{"number,title,state,url,updatedAt,author,labels,body,comments"}])?;
            if action=="pull" {
                match gh(root,&["pr","checks",&number,"--json","name,state,bucket,link,workflow"]) {
                    Ok(checks)=>detail["checks"]=checks,
                    Err(error)=>{detail["checks"]=json!([]);detail["checksError"]=json!(error.message);}
                }
                let repository=gh(root,&["repo","view","--json","nameWithOwner"])?;
                let name=repository["nameWithOwner"].as_str().ok_or_else(||HostError::new("GITHUB_RESPONSE","Repository identity missing"))?;
                let pages=gh(root,&["api",&format!("repos/{name}/pulls/{number}/comments"),"--paginate","--slurp"])?;
                detail["reviewComments"]=json!(pages.as_array().into_iter().flatten().filter_map(Value::as_array).flatten().map(|item|json!({"author":{"login":item["user"]["login"]},"body":item["body"],"path":item["path"],"line":item.get("line").filter(|value|!value.is_null()).unwrap_or(&item["original_line"]),"createdAt":item["created_at"]})).collect::<Vec<_>>());
            }
            Ok(detail)
        },
        "failedLogs" => {
            let number=number(request)?;
            let checks=gh(root,&["pr","checks",&number,"--json","name,state,bucket,link,workflow"])?;
            let mut logs=Vec::new();
            for check in checks.as_array().ok_or_else(||HostError::new("GITHUB_RESPONSE","Expected checks array"))?.iter().filter(|check|check["bucket"]=="fail").take(5) {
                let link=check["link"].as_str().unwrap_or("");
                let run_id=link.split("/actions/runs/").nth(1).and_then(|tail|tail.split('/').next()).filter(|id|!id.is_empty()&&id.bytes().all(|byte|byte.is_ascii_digit()));
                let log=match run_id {Some(id)=>run(root,"gh",&["run","view",id,"--log-failed"]).unwrap_or_else(|error|error.message),None=>link.to_string()};
                let tail=log.lines().rev().take(150).collect::<Vec<_>>().into_iter().rev().collect::<Vec<_>>().join("\n");
                logs.push(json!({"name":check["name"],"log":tail.chars().rev().take(20000).collect::<String>().chars().rev().collect::<String>()}));
            }
            Ok(json!(logs))
        },
        "createPull" => {
            let head=request_string(request,"head")?;
            let base=request_string(request,"base")?;
            for branch in [head,base] {
                if branch.starts_with('-') || branch.len()>255 {return Err(HostError::new("INVALID_ARGUMENT","Invalid branch name"));}
                git(root,&["check-ref-format","--branch",branch])?;
            }
            let title=request_string(request,"title")?;
            if title.trim().is_empty() || title.len()>256 {return Err(HostError::new("INVALID_ARGUMENT","Invalid pull request title"));}
            let file=body_file(request)?;
            let path=file.path().to_string_lossy().into_owned();
            let mut arguments=vec!["pr","create","--head",head,"--base",base,"--title",title,"--body-file",&path];
            if request["draft"].as_bool().unwrap_or(false) {arguments.push("--draft");}
            git(root,&["push","-u","origin",head])?;
            let output=run(root,"gh",&arguments)?;
            Ok(json!({"url":output.lines().last().unwrap_or("").trim()}))
        },
        "comment"=>{
            let number=number(request)?;
            if request_string(request,"body")?.trim().is_empty(){return Err(HostError::new("INVALID_ARGUMENT","Comment is empty"));}
            let file=body_file(request)?;
            run(root,"gh",&["pr","comment",&number,"--body-file",&file.path().to_string_lossy()])?;
            Ok(json!({"ok":true}))
        },
        _ => Err(HostError::unsupported(&format!("github.{action}")))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn session_diff_compares_the_first_session_version() {
        let root = tempfile::tempdir().unwrap();
        std::fs::create_dir_all(root.path().join("src")).unwrap();
        std::fs::write(root.path().join("src/app.ts"), "final\n").unwrap();
        let history = json!([
            {"path": root.path().join("src/app.ts").to_string_lossy(), "version": 2, "content": "middle\n"},
            {"path": "src/app.ts", "version": 1, "content": "first\n"},
            {"path": "other.ts", "version": 0, "content": "unrelated\n"}
        ]);
        let diff = session_diff(root.path(), "src/app.ts", &history).unwrap();
        assert_eq!(diff["before"], "first\n");
        assert_eq!(diff["after"], "final\n");
        assert!(diff["patch"].as_str().unwrap().contains("-first"));
        assert_eq!(
            session_diff(root.path(), "missing.ts", &history).unwrap_err().code,
            "NO_SESSION_VERSION"
        );
    }
    #[test]
    fn metadata_preserves_unicode() {
        let parsed=commit("abc\0one two\0Исправление\0Author\02026-10-23\0HEAD -> main");
        assert_eq!(parsed["title"], "Исправление");
        assert_eq!(parsed["parents"],json!(["one","two"]));
    }
    #[test]
    fn actual_repository_history_and_changes() {
        let directory=tempfile::tempdir().unwrap();let root=directory.path();
        git(root,&["init","-q"]).unwrap();
        std::fs::write(root.join("source.txt"),"before").unwrap();
        git(root,&["add","source.txt"]).unwrap();
        git(root,&["-c","user.name=Fixture","-c","user.email=fixture@example.test","commit","-qm","Initial"]).unwrap();
        std::fs::write(root.join("source.txt"),"after").unwrap();
        assert_eq!(changes(root).unwrap()[0]["path"],"source.txt");
        let history=dispatch(root,"github",&[json!("workspace"),json!({"action":"history"})]).unwrap();
        assert_eq!(history["commits"][0]["title"],"Initial");
        assert_eq!(history["clean"],false);
        let diff=dispatch(root,"diff",&[json!("workspace"),Value::Null,json!("source.txt"),json!("git")]).unwrap();
        assert_eq!(diff["before"],"before");
        assert_eq!(diff["after"],"after");
        assert!(diff["patch"].as_str().unwrap().contains("-before"));
    }
}
