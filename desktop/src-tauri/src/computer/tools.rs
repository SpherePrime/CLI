use serde_json::{json, Value};
use tauri::Manager;

fn tool(name: &str, description: &str, properties: Value, required: &[&str]) -> Value {
    json!({"name":name,"description":description,"inputSchema":{"type":"object","properties":properties,"required":required,"additionalProperties":false}})
}

pub fn definitions() -> Vec<Value> {
    vec![
    tool("window_list","List visible Windows desktop windows with handles, titles, process IDs, identities and bounds. Does not grant screenshots or control. Copy the selected window fields exactly into window_request_access.",json!({}),&[]),
    tool("window_request_access","Grant this chat view or control of one selected desktop window through the normal chat permission policy, including full access mode. Execution means that policy approved the request; successful results grant access immediately.",json!({"hwnd":{"type":"string"},"pid":{"type":"integer","minimum":1},"identity":{"type":"string"},"title":{"type":"string"},"mode":{"type":"string","enum":["view","control"]}}),&["hwnd","pid","identity","title","mode"]),
    tool("window_view","Capture only the window explicitly shared with this active chat. Requires view or control access. Scale image coordinates using returned imageSize and actual window bounds. Does not move focus.",json!({}),&[]),
    tool("window_action","Control only the window granted to this active chat. Input requires the selected foreground window and unobscured points; focus is never changed implicitly. Coordinates are relative to actual window bounds. Ctrl+Alt+Escape revokes all window access.",json!({"action":{"type":"string","enum":["focus","click","type","press_key","scroll"]},"x":{"type":"integer","minimum":0,"maximum":100000},"y":{"type":"integer","minimum":0,"maximum":100000},"text":{"type":"string","maxLength":1000},"key":{"type":"string","maxLength":60},"button":{"type":"string","enum":["left","right"]},"count":{"type":"integer","minimum":1,"maximum":2},"delta":{"type":"integer","minimum":-2400,"maximum":2400}}),&["action"]),
]
}

pub fn run(
    app: &tauri::AppHandle,
    context: &crate::browser::workbench::WorkbenchContext,
    name: &str,
    args: &Value,
) -> Result<Value, String> {
    app.state::<crate::state::AppState>()
        .windows
        .tool(app, context, name, args)
}
