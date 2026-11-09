use super::{
    evaluate::{self, text},
    panel::BrowserState,
    screenshot::{self, Target},
};
use serde::Deserialize;
use serde_json::{json, Value};
use std::{
    sync::atomic::{AtomicBool, Ordering},
    thread,
    time::{Duration, Instant},
};
use tauri::{AppHandle, Webview};

const MAX_STEPS: usize = 80;
const MAX_SCREENSHOTS: usize = 16;
const MAX_WAIT: Duration = Duration::from_secs(15);
const POLL: Duration = Duration::from_millis(100);

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Plan {
    pub steps: Vec<Step>,
    #[serde(default)]
    pub continue_on_error: bool,
}

#[derive(Debug, Default)]
pub struct Locate {
    pub selector: Option<String>,
    pub text: Option<String>,
}

fn locate(selector: &Option<String>, text: &Option<String>) -> Locate {
    Locate {
        selector: selector.clone(),
        text: text.clone(),
    }
}

#[derive(Debug, Deserialize)]
#[serde(
    tag = "action",
    rename_all = "snake_case",
    rename_all_fields = "camelCase",
    deny_unknown_fields
)]
pub enum Step {
    Navigate {
        url: String,
    },
    Resize {
        width: f64,
    },
    Click {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
    Type {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        value: String,
        #[serde(default)]
        submit: bool,
    },
    PressKey {
        key: String,
    },
    Scroll {
        #[serde(default)]
        to: Option<String>,
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
    WaitFor {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        #[serde(default)]
        gone: bool,
        #[serde(default)]
        timeout_ms: Option<u64>,
    },
    Wait {
        ms: u64,
    },
    Assert {
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
        #[serde(default)]
        contains: Option<String>,
        #[serde(default)]
        absent: bool,
    },
    Evaluate {
        expression: String,
    },
    Screenshot {
        name: String,
        #[serde(default)]
        full_page: bool,
        #[serde(default)]
        selector: Option<String>,
        #[serde(default)]
        text: Option<String>,
    },
}

impl Step {
    fn name(&self) -> &'static str {
        match self {
            Self::Navigate { .. } => "navigate",
            Self::Resize { .. } => "resize",
            Self::Click { .. } => "click",
            Self::Type { .. } => "type",
            Self::PressKey { .. } => "press_key",
            Self::Scroll { .. } => "scroll",
            Self::WaitFor { .. } => "wait_for",
            Self::Wait { .. } => "wait",
            Self::Assert { .. } => "assert",
            Self::Evaluate { .. } => "evaluate",
            Self::Screenshot { .. } => "screenshot",
        }
    }
}

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

pub fn parse(args: &Value) -> Result<Plan, String> {
    let plan: Plan = serde_json::from_value(json!({
        "steps": args["steps"].clone(),
        "continueOnError": args["continueOnError"].as_bool().unwrap_or(false),
    }))
    .map_err(|error| format!("Invalid steps: {error}"))?;
    if plan.steps.is_empty() {
        return Err("steps must contain at least one step".to_owned());
    }
    if plan.steps.len() > MAX_STEPS {
        return Err(format!("At most {MAX_STEPS} steps per run"));
    }
    let shots = plan
        .steps
        .iter()
        .filter(|step| matches!(step, Step::Screenshot { .. }))
        .count();
    if shots > MAX_SCREENSHOTS {
        return Err(format!("At most {MAX_SCREENSHOTS} screenshots per run"));
    }
    Ok(plan)
}

pub fn run(browser: &Browser, plan: Plan) -> Result<Value, String> {
    let started = Instant::now();
    let mut log = Vec::new();
    let mut images = Vec::new();
    let mut failed = 0;
    let mut stopped = false;
    for (index, step) in plan.steps.iter().enumerate() {
        if browser.cancel.load(Ordering::Relaxed) {
            log.push(format!("{}. stopped: cancelled", index + 1));
            stopped = true;
            break;
        }
        let begun = Instant::now();
        let outcome = execute(browser, step, &mut images);
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
    let verdict = if failed == 0 && !stopped { "PASSED" } else { "FAILED" };
    let mut summary = format!(
        "Scenario {verdict}: {} of {} steps run, {failed} failed, {} screenshots, {}ms.\n{}",
        log.len(),
        plan.steps.len(),
        images.len() / 2,
        started.elapsed().as_millis(),
        log.join("\n")
    );
    if skipped > 0 {
        summary.push_str(&format!("\n{skipped} remaining steps were not run."));
    }
    let mut content = vec![text(summary)];
    content.extend(images);
    Ok(Value::Array(content))
}

fn suffix(detail: &str) -> String {
    if detail.is_empty() {
        String::new()
    } else {
        format!(": {detail}")
    }
}

fn execute(browser: &Browser, step: &Step, images: &mut Vec<Value>) -> Result<String, String> {
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
            let target = &locate(selector, text);
            let selector = resolve(view, target, true)?;
            evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); element.scrollIntoView({{block:'center'}}); element.click(); return true }})()"))?;
            (browser.settle)()?;
            Ok(String::new())
        }
        Step::Type { selector, text, value, submit } => {
            let target = &locate(selector, text);
            let selector = resolve(view, target, true)?;
            evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); element.scrollIntoView({{block:'center'}}); element.focus(); if (element.select) element.select(); return true }})()"))?;
            super::platform::insert_text(view, value)?;
            if *submit {
                super::platform::press_key(view, "Enter")?;
            }
            (browser.settle)()?;
            Ok(String::new())
        }
        Step::PressKey { key } => {
            super::platform::press_key(view, key)?;
            (browser.settle)()?;
            Ok(String::new())
        }
        Step::Scroll { to, selector, text } => {
            let target = &locate(selector, text);
            if target.selector.is_some() || target.text.is_some() {
                let selector = resolve(view, target, false)?;
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
                let present = find(view, target, false)?.is_some();
                if present != *gone {
                    return Ok(String::new());
                }
                if Instant::now() >= deadline || browser.cancel.load(Ordering::Relaxed) {
                    let state = if *gone { "still present" } else { "not found" };
                    return Err(format!("{} {state} after {}ms", describe(target), timeout.as_millis()));
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
            let found = find(view, target, false)?;
            match (found, absent) {
                (Some(_), true) => Err(format!("{} is present", describe(target))),
                (None, true) => Ok(String::new()),
                (None, false) => Err(format!("{} not found", describe(target))),
                (Some(selector), false) => {
                    let Some(expected) = contains else { return Ok(String::new()) };
                    let actual = evaluate::immediate(view, &format!("(document.querySelector({selector}).innerText || document.querySelector({selector}).value || '').replace(/\\s+/g, ' ').trim()"))?;
                    let actual = actual.as_str().unwrap_or_default().to_owned();
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
        Step::Screenshot { name, full_page, selector, text } => {
            let target = &locate(selector, text);
            let shot_target = if target.selector.is_some() || target.text.is_some() {
                let selector = resolve(view, target, false)?;
                let selector: String = serde_json::from_str(&selector).map_err(|error| error.to_string())?;
                Target::Element(selector)
            } else if *full_page {
                Target::FullPage
            } else {
                Target::Visible
            };
            let captured = screenshot::capture(view, browser.id, shot_target, browser.cancel)?;
            let image = captured
                .as_array()
                .and_then(|parts| parts.iter().find(|part| part["type"] == "image"))
                .cloned()
                .ok_or("Browser returned no screenshot")?;
            let width = evaluate::immediate(view, "window.innerWidth")?;
            images.push(evaluate::text(format!("Screenshot \"{name}\" at {width}px")));
            images.push(image);
            Ok(name.clone())
        }
    }
}

fn describe(target: &Locate) -> String {
    match (&target.selector, &target.text) {
        (Some(selector), _) => format!("selector {selector:?}"),
        (None, Some(text)) => format!("text {text:?}"),
        (None, None) => "target".to_owned(),
    }
}

fn resolve(view: &Webview, target: &Locate, interactive: bool) -> Result<String, String> {
    find(view, target, interactive)?.ok_or_else(|| format!("{} not found", describe(target)))
}

fn find(view: &Webview, target: &Locate, interactive: bool) -> Result<Option<String>, String> {
    let selector = serde_json::to_string(&target.selector).map_err(|error| error.to_string())?;
    let wanted = serde_json::to_string(&target.text).map_err(|error| error.to_string())?;
    if target.selector.is_none() && target.text.is_none() {
        return Err("Give selector or text".to_owned());
    }
    let found = evaluate::immediate(view, &format!("(() => {{ const selector = {selector}; const wanted = {wanted}; const visible = element => {{ const box = element.getBoundingClientRect(); const style = getComputedStyle(element); return box.width > 0 && box.height > 0 && style.visibility !== 'hidden' && style.display !== 'none' }}; let element = null; if (selector) {{ element = [...document.querySelectorAll(selector)].find(visible) || null }} else {{ const needle = wanted.replace(/\\s+/g, ' ').trim().toLowerCase(); const label = item => (item.innerText || item.value || item.getAttribute('aria-label') || item.getAttribute('placeholder') || item.getAttribute('title') || '').replace(/\\s+/g, ' ').trim().toLowerCase(); const pool = [...document.querySelectorAll({interactive_pool})].filter(visible); element = pool.find(item => label(item) === needle) || pool.find(item => label(item).includes(needle)) || null; if (!element && !{interactive}) {{ const all = [...document.querySelectorAll('body *')].filter(visible).filter(item => label(item).includes(needle)); element = all.find(item => ![...item.children].some(child => all.includes(child))) || null }} }} if (!element) return null; const marker = String(Date.now()) + Math.random().toString(16).slice(2); element.setAttribute('data-prime-run', marker); return marker }})()", interactive_pool = serde_json::to_string(INTERACTIVE).map_err(|error| error.to_string())?))?;
    Ok(found
        .as_str()
        .map(|marker| serde_json::to_string(&format!("[data-prime-run=\"{marker}\"]")))
        .transpose()
        .map_err(|error| error.to_string())?)
}

const INTERACTIVE: &str = "a,button,input,textarea,select,summary,label,[role=button],[role=link],[role=tab],[role=menuitem],[role=checkbox],[role=switch],[onclick],[tabindex]";

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_a_full_scenario() {
        let plan = parse(&json!({"steps":[
            {"action":"navigate","url":"http://localhost:5173"},
            {"action":"resize","width":390},
            {"action":"click","text":"Записаться"},
            {"action":"type","selector":"#email","value":"a@b.c","submit":true},
            {"action":"wait_for","text":"Готово","timeoutMs":3000},
            {"action":"assert","selector":"h1","contains":"Prime"},
            {"action":"scroll","to":"bottom"},
            {"action":"screenshot","name":"mobile","fullPage":true},
            {"action":"screenshot","name":"form","selector":"form"}
        ]}))
        .unwrap();
        assert_eq!(plan.steps.len(), 9);
        assert!(!plan.continue_on_error);
        assert!(matches!(&plan.steps[2], Step::Click { text, .. } if text.as_deref() == Some("Записаться")));
        assert!(matches!(&plan.steps[7], Step::Screenshot { full_page: true, .. }));
    }

    #[test]
    fn rejects_bad_scenarios() {
        assert!(parse(&json!({"steps":[]})).is_err());
        assert!(parse(&json!({"steps":[{"action":"fly"}]})).is_err());
        assert!(parse(&json!({"steps":[{"action":"click","selector":"a","extra":1}]})).is_err());
        let shots: Vec<Value> = (0..=MAX_SCREENSHOTS).map(|index| json!({"action":"screenshot","name":index.to_string()})).collect();
        assert!(parse(&json!({"steps":shots})).is_err());
    }
}
