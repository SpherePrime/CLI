pub fn build_system_prompt(working_dir: &str, tools: &[String]) -> String {
    let tool_list = if tools.is_empty() {
        "No tools are available; answer from your own knowledge.".to_string()
    } else {
        format!("Available tools: {}.", tools.join(", "))
    };
    format!(
        "You are an autonomous coding agent running in the user's terminal.\n\
         Working directory: {working_dir}\n\
         {tool_list}\n\n\
         Rules:\n\
         - Prefer using tools to inspect and modify files instead of guessing.\n\
         - Read a file before editing it.\n\
         - Never invent file contents or command output.\n\
         - Keep answers concise and report what you changed.\n\
         - Paths are relative to the working directory; absolute paths and `..` escapes are rejected.\n\
         - When a tool fails, explain the failure and try a different approach.\n\
         - When the task is complete, respond with a short summary and no tool calls."
    )
}
