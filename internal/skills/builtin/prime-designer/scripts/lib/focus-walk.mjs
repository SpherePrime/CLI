const FOCUS_STYLE_KEYS = ["outlineStyle", "outlineWidth", "outlineColor", "boxShadow", "backgroundColor", "color", "borderColor", "textDecorationLine"];

function snapshotFocused(keys) {
  let element = document.activeElement;
  while (element?.shadowRoot?.activeElement) element = element.shadowRoot.activeElement;
  if (!element || element === document.body) return null;
  if (!element.dataset.qaFocusId) {
    window.__qaFocusCounter = (window.__qaFocusCounter ?? 0) + 1;
    element.dataset.qaFocusId = String(window.__qaFocusCounter);
  }
  const style = getComputedStyle(element);
  const rect = element.getBoundingClientRect();
  const root = element.getRootNode();
  const hit = (typeof root.elementFromPoint === "function" ? root : document).elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
  const composedContains = (outer, inner) => {
    for (let node = inner; node; node = node.parentNode || node.host) if (node === outer) return true;
    return false;
  };
  const center = hit && (composedContains(element, hit) || composedContains(hit, element)) ? null : hit;
  const label = element.id
    ? `#${element.id}`
    : `${element.tagName.toLowerCase()}${element.className && typeof element.className === "string" ? "." + element.className.trim().split(/\s+/).slice(0, 2).join(".") : ""}`;
  return {
    identity: element.dataset.qaFocusId,
    label,
    text: (element.innerText || element.getAttribute("aria-label") || element.value || "").trim().slice(0, 50),
    style: Object.fromEntries(keys.map((key) => [key, style[key]])),
    rect: { x: Math.round(rect.left), y: Math.round(rect.top), w: Math.round(rect.width), h: Math.round(rect.height) },
    inViewport: rect.bottom > 0 && rect.right > 0 && rect.top < innerHeight && rect.left < innerWidth,
    visible: rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && Number(style.opacity) > 0,
    obscured: Boolean(center),
    obscuredBy: center ? center.tagName.toLowerCase() + (center.id ? "#" + center.id : "") : null,
  };
}

function snapshotBlurred(keys) {
  let element = document.activeElement;
  while (element?.shadowRoot?.activeElement) element = element.shadowRoot.activeElement;
  if (!element || element === document.body) return null;
  element.blur();
  const style = getComputedStyle(element);
  return Object.fromEntries(keys.map((key) => [key, style[key]]));
}

function hasIndicator(focused, blurred) {
  const outline = focused.outlineStyle !== "none" && parseFloat(focused.outlineWidth) > 0;
  if (outline) return true;
  if (!blurred) return focused.boxShadow !== "none";
  return FOCUS_STYLE_KEYS.some((key) => key !== "outlineWidth" && key !== "outlineStyle" && focused[key] !== blurred[key]);
}

export async function walkFocus(page, maxStops) {
  const issues = [];
  const order = [];
  await page.evaluate(() => {
    scrollTo(0, 0);
    document.activeElement?.blur();
  });
  let repeats = 0;
  let previousIdentity = null;
  let firstIdentity = null;
  for (let stop = 0; stop < maxStops; stop += 1) {
    await page.keyboard.press("Tab");
    const focused = await page.evaluate(snapshotFocused, FOCUS_STYLE_KEYS);
    if (!focused) {
      if (stop === 0) issues.push({ rule: "focus-none", severity: "warn", message: "Tab does not reach any element from the top of the page" });
      break;
    }
    if (stop > 0 && focused.identity === firstIdentity) break;
    firstIdentity ??= focused.identity;
    const blurred = await page.evaluate(snapshotBlurred, FOCUS_STYLE_KEYS);
    order.push(`${focused.label}${focused.text ? ` "${focused.text}"` : ""}`);
    const where = { selector: focused.label, text: focused.text, rect: focused.rect };
    if (!focused.visible) {
      issues.push({ rule: "focus-invisible-stop", severity: "error", message: "Keyboard focus lands on an invisible element (hidden menu item, offscreen control)", ...where });
    } else if (!focused.inViewport) {
      issues.push({ rule: "focus-offscreen", severity: "error", message: "Focused element is outside the viewport after focusing", ...where });
    } else if (focused.obscured) {
      issues.push({ rule: "focus-obscured", severity: "error", message: `Focused element is covered by ${focused.obscuredBy} (WCAG 2.4.11)`, ...where });
    }
    if (focused.visible && !hasIndicator(focused.style, blurred)) {
      issues.push({ rule: "focus-indicator", severity: "error", message: "No visible focus indicator (outline, ring, or style change)", ...where });
    }
    repeats = focused.identity === previousIdentity ? repeats + 1 : 0;
    if (repeats >= 2) {
      issues.push({ rule: "focus-trap", severity: "error", message: "Focus is stuck on the same element", ...where });
      break;
    }
    previousIdentity = focused.identity;
  }
  return { order, issues };
}
