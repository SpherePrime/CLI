use super::evaluate;
use serde_json::Value;
use tauri::Webview;

#[derive(Debug, Default)]
pub struct Locate {
    pub selector: Option<String>,
    pub text: Option<String>,
}

pub fn locate(selector: &Option<String>, text: &Option<String>) -> Locate {
    Locate {
        selector: selector.clone(),
        text: text.clone(),
    }
}

pub fn describe(target: &Locate) -> String {
    match (&target.selector, &target.text) {
        (Some(selector), _) => format!("selector {selector:?}"),
        (None, Some(text)) => format!("text {text:?}"),
        (None, None) => "target".to_owned(),
    }
}

pub fn resolve(view: &Webview, target: &Locate, interactive: bool) -> Result<String, String> {
    find(view, target, interactive)?.ok_or_else(|| format!("{} not found", describe(target)))
}

pub fn find(view: &Webview, target: &Locate, interactive: bool) -> Result<Option<String>, String> {
    let selector = serde_json::to_string(&target.selector).map_err(|error| error.to_string())?;
    let wanted = serde_json::to_string(&target.text).map_err(|error| error.to_string())?;
    if target.selector.is_none() && target.text.is_none() {
        return Err("Give selector or text".to_owned());
    }
    let found = evaluate::immediate(view, &format!("(() => {{ const selector = {selector}; const wanted = {wanted}; const visible = element => {{ const box = element.getBoundingClientRect(); const style = getComputedStyle(element); return box.width > 0 && box.height > 0 && style.visibility !== 'hidden' && style.display !== 'none' }}; let element = null; if (selector) {{ element = [...document.querySelectorAll(selector)].find(visible) || null }} else {{ const needle = wanted.replace(/\\s+/g, ' ').trim().toLowerCase(); const label = item => (item.innerText || item.value || item.getAttribute('aria-label') || item.getAttribute('placeholder') || item.getAttribute('title') || '').replace(/\\s+/g, ' ').trim().toLowerCase(); const pool = [...document.querySelectorAll({interactive_pool})].filter(visible); element = pool.find(item => label(item) === needle) || pool.find(item => label(item).includes(needle)) || null; if (!element && !{interactive}) {{ const all = [...document.querySelectorAll('body *')].filter(visible).filter(item => label(item).includes(needle)); element = all.find(item => ![...item.children].some(child => all.includes(child))) || null }} }} if (!element) return null; const marker = String(Date.now()) + Math.random().toString(16).slice(2); element.setAttribute('data-prime-run', marker); return marker }})()", interactive_pool = serde_json::to_string(INTERACTIVE).map_err(|error| error.to_string())?))?;
    found
        .as_str()
        .map(|marker| serde_json::to_string(&format!("[data-prime-run=\"{marker}\"]")))
        .transpose()
        .map_err(|error| error.to_string())
}

pub struct Point {
    pub x: f64,
    pub y: f64,
    pub covered_by: Option<String>,
}

pub fn center(view: &Webview, selector: &str) -> Result<Point, String> {
    let point = evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); element.scrollIntoView({{block:'center', inline:'center'}}); const box = element.getBoundingClientRect(); const x = box.left + box.width / 2; const y = box.top + box.height / 2; const hit = document.elementFromPoint(x, y); const covered = hit && hit !== element && !element.contains(hit) ? hit.tagName.toLowerCase() + (hit.id ? '#' + hit.id : '') + (typeof hit.className === 'string' && hit.className.trim() ? '.' + hit.className.trim().split(/\\s+/).slice(0, 2).join('.') : '') : null; return {{ x, y, covered }} }})()"))?;
    let coordinate = |name: &str| {
        point[name]
            .as_f64()
            .filter(|value| value.is_finite())
            .ok_or_else(|| "Element has no position on the page".to_owned())
    };
    Ok(Point {
        x: coordinate("x")?,
        y: coordinate("y")?,
        covered_by: point["covered"].as_str().map(str::to_owned),
    })
}

pub fn dom_click(view: &Webview, selector: &str) -> Result<(), String> {
    evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); element.scrollIntoView({{block:'center'}}); element.click(); return true }})()"))?;
    Ok(())
}

pub fn dom_hover(view: &Webview, selector: &str) -> Result<(), String> {
    evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); for (const type of ['pointerover', 'pointerenter', 'mouseover', 'mouseenter', 'mousemove']) element.dispatchEvent(new MouseEvent(type, {{ bubbles: !type.endsWith('enter'), cancelable: true }})); return true }})()"))?;
    Ok(())
}

pub fn focus(view: &Webview, selector: &str) -> Result<(), String> {
    evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); element.scrollIntoView({{block:'center'}}); element.focus(); return document.activeElement === element }})()"))?;
    Ok(())
}

pub fn select_option(view: &Webview, selector: &str, option: &str) -> Result<String, String> {
    let wanted = serde_json::to_string(option).map_err(|error| error.to_string())?;
    let result = evaluate::immediate(view, &format!("(() => {{ const element = document.querySelector({selector}); if (!(element instanceof HTMLSelectElement)) return {{ error: 'target is not a <select>; click the custom dropdown and its option instead' }}; const wanted = {wanted}.trim().toLowerCase(); const options = [...element.options]; const match = options.find(item => item.value.toLowerCase() === wanted) || options.find(item => item.text.trim().toLowerCase() === wanted) || options.find(item => item.text.trim().toLowerCase().includes(wanted)); if (!match) return {{ error: 'option not found; available: ' + options.slice(0, 20).map(item => item.text.trim() || item.value).join(', ') }}; element.value = match.value; element.dispatchEvent(new Event('input', {{ bubbles: true }})); element.dispatchEvent(new Event('change', {{ bubbles: true }})); return {{ selected: match.text.trim() || match.value }} }})()"))?;
    match (result["selected"].as_str(), result["error"].as_str()) {
        (Some(selected), _) => Ok(selected.to_owned()),
        (None, Some(error)) => Err(error.to_owned()),
        _ => Err("Could not select the option".to_owned()),
    }
}

pub fn text_of(view: &Webview, selector: &str) -> Result<String, String> {
    let actual = evaluate::immediate(view, &format!("(document.querySelector({selector}).innerText || document.querySelector({selector}).value || '').replace(/\\s+/g, ' ').trim()"))?;
    Ok(actual.as_str().unwrap_or_default().to_owned())
}

pub fn unquote(selector: &str) -> Result<String, String> {
    serde_json::from_str::<Value>(selector)
        .ok()
        .and_then(|value| value.as_str().map(str::to_owned))
        .ok_or_else(|| "Invalid element selector".to_owned())
}

const INTERACTIVE: &str = "a,button,input,textarea,select,summary,label,[role=button],[role=link],[role=tab],[role=menuitem],[role=checkbox],[role=switch],[onclick],[tabindex]";
