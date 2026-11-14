use super::{
    evaluate::{self, text_part},
    panel::BrowserState,
    platform,
    scenario_dom::{self as dom, locate},
    screenshot::{self, Shot, Target},
};
use serde_json::Value;
use std::{
    sync::atomic::{AtomicBool, Ordering},
    thread,
    time::{Duration, Instant},
};
use tauri::{AppHandle, Webview};

pub use super::scenario_parse::{parse, Plan, Step};

const MAX_WAIT: Duration = Duration::from_secs(15);
const POLL: Duration = Duration::from_millis(100);
const HOVER_SETTLE: Duration = Duration::from_millis(350);

pub struct Browser<'a> {
    pub app: &'a AppHandle,
    pub state: &'a BrowserState,
    pub session: Option<&'a str>,
    pub id: &'a str,
    pub view: &'a Webview,
    pub cancel: &'a AtomicBool,
    pub settle: &'a dyn Fn() -> Result<(), String>,
    pub resize: &'a dyn Fn(f64) -> Result<(), String>,
}

pub fn run(browser: &Browser, plan: Plan) -> Result<Value, String> {
    let started = Instant::now();
    let mut log = Vec::new();
    let mut images = Vec::new();
    let mut shots = 0;
    let mut failed = 0;
    let mut stopped = false;
    for (index, step) in plan.steps.iter().enumerate() {
        if browser.cancel.load(Ordering::Relaxed) {
            log.push(format!("{}. stopped: cancelled", index + 1));
            stopped = true;
            break;
        }
        let begun = Instant::now();
        let outcome = execute(browser, step, &mut images, &mut shots);
        let elapsed = begun.elapsed().as_millis();
        match outcome {
            Ok(detail) => log.push(format!("{}. {} ok {elapsed}ms{}", index + 1, step.name(), suffix(&detail))),
            Err(error) => {
                failed += 1;
                log.push(format!("{}. {} FAILED {elapsed}ms: {error}", index + 1, step.name()));
                let is_check = matches!(step, Step::Assert { .. });
                if !is_check && !plan.continue_on_error {
                    stopped = true;
                    break;
                }
            }
        }
    }
    let skipped = plan.steps.len() - log.len();
    let summary = build_summary(
        &log,
        plan.steps.len(),
        failed,
        stopped,
        shots,
        started.elapsed().as_millis(),
        skipped,
        &plan.notes,
    );
    Ok(assemble_content(summary, images))
}

fn build_summary(
    log: &[String],
    total_steps: usize,
    failed: usize,
    stopped: bool,
    shots: usize,
    elapsed_ms: u128,
    skipped: usize,
    notes: &[String],
) -> String {
    let verdict = if failed == 0 && !stopped { "PASSED" } else { "FAILED" };
    let mut summary = format!(
        "Scenario {verdict}: {} of {total_steps} steps run, {failed} failed, {shots} screenshots, {elapsed_ms}ms.\n{}",
        log.len(),
        log.join("\n")
    );
    if skipped > 0 {
        summary.push_str(&format!("\n{skipped} remaining steps were not run."));
    }
    if !notes.is_empty() {
        summary.push_str(&format!("\nNotes:\n{}", notes.join("\n")));
    }
    summary
}

fn assemble_content(summary: String, images: Vec<Value>) -> Value {
    let mut content = vec![text_part(summary)];
    content.extend(images);
    Value::Array(content)
}

fn suffix(detail: &str) -> String {
    if detail.is_empty() {
        String::new()
    } else {
        format!(": {detail}")
    }
}

fn execute(browser: &Browser, step: &Step, images: &mut Vec<Value>, shots: &mut usize) -> Result<String, String> {
    let view = browser.view;
    match step {
        Step::Navigate { url } => {
            browser.state.navigate(browser.app, url, Some(browser.id))?;
            (browser.settle)()?;
            Ok(String::new())
        }
        Step::Resize { width } => {
            (browser.resize)(*width)?;
            Ok(format!("{}px", evaluate::immediate(view, "window.innerWidth")?))
        }
        Step::Click { selector, text } => {
            let selector = dom::resolve(view, &locate(selector, text), true)?;
            let detail = click(view, &selector)?;
            (browser.settle)()?;
            Ok(detail)
        }
        Step::Hover { selector, text } => {
            let selector = dom::resolve(view, &locate(selector, text), false)?;
            let point = dom::center(view, &selector)?;
            let detail = match platform::pointer(view, point.x, point.y, false) {
                Ok(()) => covered_note(&point.covered_by),
                Err(_) => {
                    dom::dom_hover(view, &selector)?;
                    "synthetic events only, CSS :hover is not applied".to_owned()
                }
            };
            thread::sleep(HOVER_SETTLE);
            Ok(detail)
        }
        Step::Focus { selector, text } => {
            let selector = dom::resolve(view, &locate(selector, text), false)?;
            dom::focus(view, &selector)?;
            thread::sleep(Duration::from_millis(150));
            Ok(String::new())
        }
        Step::Type { selector, text, value, submit } => {
            let selector = dom::resolve(view, &locate(selector, text), true)?;
            evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); element.scrollIntoView({{block:'center'}}); element.focus(); if (element.select) element.select(); return true }})()"))?;
            platform::insert_text(view, value)?;
            if *submit {
                platform::press_key(view, "Enter")?;
            }
            (browser.settle)()?;
            Ok(String::new())
        }
        Step::Select { selector, text, option } => {
            let selector = dom::resolve(view, &locate(selector, text), true)?;
            let selected = dom::select_option(view, &selector, option)?;
            (browser.settle)()?;
            Ok(selected)
        }
        Step::PressKey { key } => {
            platform::press_key(view, key)?;
            (browser.settle)()?;
            Ok(String::new())
        }
        Step::Scroll { to, selector, text } => {
            let target = &locate(selector, text);
            if target.selector.is_some() || target.text.is_some() {
                let selector = dom::resolve(view, target, false)?;
                evaluate::immediate(view, &format!("document.querySelector({selector}).scrollIntoView({{block:'start'}})"))?;
            } else {
                let expression = match to.as_deref() {
                    Some("top") => "scrollTo(0, 0)",
                    Some("bottom") | None => "scrollTo(0, document.documentElement.scrollHeight)",
                    Some(other) => return Err(format!("scroll to must be top or bottom, got {other}")),
                };
                evaluate::immediate(view, expression)?;
            }
            thread::sleep(Duration::from_millis(250));
            Ok(String::new())
        }
        Step::WaitFor { selector, text, gone, timeout_ms } => {
            let target = &locate(selector, text);
            let timeout = timeout_ms.map(Duration::from_millis).unwrap_or(MAX_WAIT).min(MAX_WAIT);
            let deadline = Instant::now() + timeout;
            loop {
                let present = dom::find(view, target, false)?.is_some();
                if present != *gone {
                    return Ok(String::new());
                }
                if Instant::now() >= deadline || browser.cancel.load(Ordering::Relaxed) {
                    let state = if *gone { "still present" } else { "not found" };
                    return Err(format!("{} {state} after {}ms", dom::describe(target), timeout.as_millis()));
                }
                thread::sleep(POLL);
            }
        }
        Step::Wait { ms } => {
            thread::sleep(Duration::from_millis(*ms).min(MAX_WAIT));
            Ok(String::new())
        }
        Step::Assert { selector, text, contains, absent } => {
            let target = &locate(selector, text);
            let found = dom::find(view, target, false)?;
            match (found, absent) {
                (Some(_), true) => Err(format!("{} is present", dom::describe(target))),
                (None, true) => Ok(String::new()),
                (None, false) => Err(format!("{} not found", dom::describe(target))),
                (Some(selector), false) => {
                    let Some(expected) = contains else { return Ok(String::new()) };
                    let actual = dom::text_of(view, &selector)?;
                    if actual.to_lowercase().contains(&expected.to_lowercase()) {
                        Ok(String::new())
                    } else {
                        Err(format!("expected text containing {expected:?}, got {:?}", actual.chars().take(200).collect::<String>()))
                    }
                }
            }
        }
        Step::Evaluate { expression } => {
            let value = evaluate::evaluate_until_done(view, browser.id, expression, browser.cancel)?;
            Ok(value.to_string().chars().take(500).collect())
        }
        Step::Screenshot { name, full_page, tiles, selector, text, padding, clip, zoom } => {
            let target = &locate(selector, text);
            let shot_target = if target.selector.is_some() || target.text.is_some() {
                Target::Element(dom::unquote(&dom::resolve(view, target, false)?)?)
            } else if let Some(clip) = clip {
                Target::Region(*clip)
            } else if *full_page {
                Target::FullPage { tiles: *tiles }
            } else {
                Target::Visible
            };
            let shot = Shot::new(shot_target, *padding, *zoom)?;
            let captured = screenshot::capture(view, browser.id, &shot, browser.cancel)?;
            if !captured.iter().any(|part| part["type"] == "image") {
                return Err("Browser returned no screenshot".to_owned());
            }
            let width = evaluate::immediate(view, "window.innerWidth")?;
            images.push(text_part(format!("Screenshot \"{name}\" at {width}px")));
            images.extend(captured);
            *shots += 1;
            Ok(name.clone())
        }
    }
}

fn click(view: &Webview, selector: &str) -> Result<String, String> {
    let point = dom::center(view, selector)?;
    if point.covered_by.is_some() {
        dom::dom_click(view, selector)?;
        return Ok(format!("{}; DOM click used", covered_note(&point.covered_by)));
    }
    match platform::pointer(view, point.x, point.y, true) {
        Ok(()) => Ok(String::new()),
        Err(_) => {
            dom::dom_click(view, selector)?;
            Ok(String::new())
        }
    }
}

fn covered_note(covered_by: &Option<String>) -> String {
    covered_by
        .as_ref()
        .map(|layer| format!("element center is covered by {layer}"))
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn assembled_content_contains_only_flat_mcp_objects() {
        let passed_summary = build_summary(
            &["1. navigate ok 12ms".to_owned(), "2. screenshot ok 20ms: home".to_owned()],
            2,
            0,
            false,
            1,
            32,
            0,
            &[],
        );
        assert!(passed_summary.starts_with("Scenario PASSED: 2 of 2 steps run, 0 failed, 1 screenshots"));

        let failed_summary = build_summary(
            &["1. click FAILED 5ms: not found".to_owned()],
            3,
            1,
            true,
            1,
            15,
            2,
            &["extra note".to_owned()],
        );
        assert!(failed_summary.starts_with("Scenario FAILED: 1 of 3 steps run, 1 failed, 1 screenshots"));
        assert!(failed_summary.contains("2 remaining steps were not run."));
        assert!(failed_summary.contains("Notes:\nextra note"));

        let images = vec![
            text_part("Screenshot \"home\" at 1280px"),
            json!({ "type": "image", "data": "AAAA", "mimeType": "image/png" }),
        ];
        let content = assemble_content(failed_summary, images);
        let parts = content.as_array().expect("scenario content must be a JSON array");
        assert_eq!(parts.len(), 3);
        for part in parts {
            assert!(part.is_object(), "every content part must be a JSON object, got {part}");
            assert!(!part.is_array());
            assert!(part["type"].as_str().is_some());
        }
        assert_eq!(parts[0]["type"], "text");
        assert_eq!(parts[1]["type"], "text");
        assert_eq!(parts[2]["type"], "image");
    }
}
