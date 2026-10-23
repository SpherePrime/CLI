use serde_json::{json, Value};
use std::path::Path;

fn tool(name: &str, description: &str, properties: Value, required: &[&str]) -> Value {
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

pub fn definitions() -> Vec<Value> {
    let sha = json!({"type":"string","pattern":"^(?:[a-fA-F0-9]{40}|[a-fA-F0-9]{64})$"});
    let number = json!({"type":"integer","minimum":1});
    let mut tools = vec![
        tool("git_history", "Read existing commits from local and remote-tracking branches including HEAD.", json!({}), &[]),
        tool("git_show_commit", "Inspect an existing project commit and diff using its full SHA from git_history.", json!({"sha":sha}), &["sha"]),
        tool("git_revert_commit", "Only on explicit user request, create a local commit reversing the selected commit; merge commits use the first parent. Pass expectedHead from the latest preview. Preserve staged, unstaged and untracked work in a stash backup. Does not push.", json!({"sha":sha,"expectedHead":sha}), &["sha","expectedHead"]),
        tool("git_restore_commit", "Only on explicit user request, create a new local commit restoring the selected commit's tree while retaining history. Pass expectedHead from the latest preview. Preserve staged, unstaged and untracked work in a stash backup. Does not push.", json!({"sha":sha,"expectedHead":sha}), &["sha","expectedHead"]),
        tool("github_create_pull", "On explicit user request, push the head branch to origin and publish a GitHub pull request using the current account.", json!({"head":{"type":"string","minLength":1,"maxLength":255},"base":{"type":"string","minLength":1,"maxLength":255},"title":{"type":"string","minLength":1,"maxLength":256},"body":{"type":"string","maxLength":60000},"draft":{"type":"boolean"}}), &["head","base","title","body","draft"]),
        tool("github_comment", "On explicit user request, publish a comment on a project pull request using the current GitHub account.", json!({"number":number,"body":{"type":"string","minLength":1,"maxLength":60000}}), &["number","body"]),
    ];
    for action in [
        "status",
        "issues",
        "pulls",
        "branches",
        "issue",
        "pull",
        "diff",
        "failed_logs",
    ] {
        let detail = ["issue", "pull", "diff", "failed_logs"].contains(&action);
        tools.push(tool(
            &format!("github_{action}"),
            &format!("Read GitHub {action} for this project using its existing account."),
            if detail {
                json!({"number":number})
            } else {
                json!({})
            },
            if detail { &["number"] } else { &[] },
        ));
    }
    tools
}

fn request(name: &str, args: &Value) -> Result<Value, String> {
    let action = match name {
        "git_history" => "history",
        "git_show_commit" => "showCommit",
        "git_revert_commit" => "revertCommit",
        "git_restore_commit" => "restoreCommit",
        "github_failed_logs" => "failedLogs",
        "github_create_pull" => "createPull",
        "github_comment" => "comment",
        "github_status" => "status",
        "github_issues" => "issues",
        "github_pulls" => "pulls",
        "github_branches" => "branches",
        "github_issue" => "issue",
        "github_pull" => "pull",
        "github_diff" => "diff",
        _ => return Err("Unknown Git tool".into()),
    };
    for field in ["sha", "expectedHead"] {
        if let Some(value) = args.get(field) {
            let value = value.as_str().ok_or("Commit SHA must be a string")?;
            if ![40, 64].contains(&value.len())
                || !value.bytes().all(|byte| byte.is_ascii_hexdigit())
            {
                return Err("Commit SHA must be a full hexadecimal object ID".into());
            }
        }
    }
    if args
        .get("number")
        .is_some_and(|number| number.as_u64().filter(|number| *number > 0).is_none())
    {
        return Err("Pull request or issue number must be a positive integer".into());
    }
    if action == "createPull" {
        if !args["draft"].is_boolean() {
            return Err("draft must be boolean".into());
        }
        for field in ["head", "base", "title", "body"] {
            if !args[field].is_string() {
                return Err(format!("{field} must be a string"));
            }
        }
    }
    let mut request = args.clone();
    request["action"] = json!(action);
    Ok(request)
}

pub fn run(
    root: &Path,
    workspace: &str,
    remote: bool,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    if remote {
        return Err(
            "Remote Git tools require the SSH Git adapter; local fallback is not allowed".into(),
        );
    }
    crate::git::dispatch(root, "github", &[json!(workspace), request(name, args)?])
        .map_err(|error| error.message)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn mutation_and_publication_arguments_are_preserved() {
        let sha = "a".repeat(40);
        let value = request(
            "git_restore_commit",
            &json!({"sha":sha,"expectedHead":"b".repeat(40)}),
        )
        .unwrap();
        assert_eq!(value["action"], "restoreCommit");
        assert_eq!(value["expectedHead"], "b".repeat(40));
        assert!(request("git_revert_commit", &json!({"sha":"HEAD"})).is_err());
        assert!(request("github_issue", &json!({"number":-1})).is_err());
        assert!(request("github_create_pull", &json!({"draft":"false"})).is_err());
        assert_eq!(
            request("github_failed_logs", &json!({"number":1})).unwrap()["action"],
            "failedLogs"
        );
    }
}
