(() => {
  const CONTROL_SELECTOR = "a[href], button, input:not([type=hidden]), select, textarea, summary, [role=button], [role=link], [role=tab], [role=menuitem], [role=checkbox], [role=switch], [tabindex]:not([tabindex='-1'])";
  const MEDIA_SELECTOR = "img, video, canvas, picture, iframe, svg:not(button svg):not(a svg)";
  const TEXT_BLOCK_SELECTOR = "p, li, dd, blockquote, figcaption";
  const HEADING_SELECTOR = "h1, h2, h3, h4, h5, h6";
  const SKIP_TAGS = new Set(["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE", "HEAD", "META", "LINK", "TITLE", "BR", "OPTION"]);

  const defaults = {
    maxIssuesPerRule: 25,
    minTouchTarget: 24,
    comfortableTouchTarget: 44,
    overlapTolerance: 2,
    includeTokenReport: true,
  };

  function round(value) {
    return Math.round(value * 10) / 10;
  }

  function selectorFor(element) {
    if (!(element instanceof Element)) return "";
    if (element.id) return `#${CSS.escape(element.id)}`;
    const parts = [];
    let current = element;
    while (current && current.nodeType === 1 && parts.length < 4) {
      let part = current.tagName.toLowerCase();
      if (current.id) {
        parts.unshift(`#${CSS.escape(current.id)}`);
        break;
      }
      const classes = [...current.classList].filter((name) => !/^(is-|has-|js-)/.test(name)).slice(0, 2);
      if (classes.length) part += `.${classes.map((name) => CSS.escape(name)).join(".")}`;
      const parent = current.parentElement;
      if (parent) {
        const sameTag = [...parent.children].filter((child) => child.tagName === current.tagName);
        if (sameTag.length > 1) part += `:nth-of-type(${sameTag.indexOf(current) + 1})`;
      }
      parts.unshift(part);
      current = parent;
    }
    return parts.join(" > ");
  }

  function textPreview(element) {
    return (element.innerText || element.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60);
  }

  function isVisible(element) {
    if (typeof element.checkVisibility === "function") {
      if (!element.checkVisibility({ opacityProperty: true, visibilityProperty: true, contentVisibilityAuto: true })) return false;
    } else {
      const style = getComputedStyle(element);
      if (style.visibility === "hidden" || style.display === "none" || Number(style.opacity) === 0) return false;
    }
    const rect = element.getBoundingClientRect();
    if (rect.width <= 1 || rect.height <= 1) return false;
    return rect.bottom + scrollY > 0 && rect.right + scrollX > 0;
  }

  function toPageRect(rect) {
    return {
      left: rect.left + scrollX,
      top: rect.top + scrollY,
      right: rect.right + scrollX,
      bottom: rect.bottom + scrollY,
      width: rect.width,
      height: rect.height,
    };
  }

  function intersectRects(a, b) {
    const left = Math.max(a.left, b.left);
    const top = Math.max(a.top, b.top);
    const right = Math.min(a.right, b.right);
    const bottom = Math.min(a.bottom, b.bottom);
    if (right <= left || bottom <= top) return null;
    return { left, top, right, bottom, width: right - left, height: bottom - top };
  }

  function clipsOverflow(style) {
    return ["hidden", "clip", "auto", "scroll"].includes(style.overflowX) || ["hidden", "clip", "auto", "scroll"].includes(style.overflowY);
  }

  function visibleRect(element, rect) {
    let clipped = rect;
    let ancestor = element.parentElement;
    while (ancestor && ancestor !== document.body && ancestor !== document.documentElement) {
      const style = getComputedStyle(ancestor);
      if (style.position === "fixed") break;
      if (clipsOverflow(style)) {
        clipped = intersectRects(clipped, toPageRect(ancestor.getBoundingClientRect()));
        if (!clipped) return null;
      }
      ancestor = ancestor.parentElement;
    }
    return clipped;
  }

  function ownTextNodes(element) {
    return [...element.childNodes].filter((node) => node.nodeType === 3 && node.textContent.trim().length > 0);
  }

  function textRect(element) {
    const nodes = ownTextNodes(element);
    if (!nodes.length) return null;
    const range = document.createRange();
    let union = null;
    for (const node of nodes) {
      range.selectNodeContents(node);
      for (const lineRect of range.getClientRects()) {
        if (lineRect.width < 1 || lineRect.height < 1) continue;
        const pageRect = toPageRect(lineRect);
        union = union
          ? {
              left: Math.min(union.left, pageRect.left),
              top: Math.min(union.top, pageRect.top),
              right: Math.max(union.right, pageRect.right),
              bottom: Math.max(union.bottom, pageRect.bottom),
            }
          : pageRect;
      }
    }
    if (!union) return null;
    union.width = union.right - union.left;
    union.height = union.bottom - union.top;
    return union;
  }

  function hasFixedAncestor(element) {
    let current = element;
    while (current && current !== document.documentElement) {
      const position = getComputedStyle(current).position;
      if (position === "fixed" || position === "sticky") return true;
      current = current.parentElement;
    }
    return false;
  }

  function parseColor(value) {
    const match = /rgba?\(([^)]+)\)/.exec(value);
    if (!match) return null;
    const parts = match[1].split(/[\s,/]+/).filter(Boolean).map(Number);
    return { r: parts[0], g: parts[1], b: parts[2], a: parts.length > 3 ? parts[3] : 1 };
  }

  function blend(top, bottom) {
    const alpha = top.a + bottom.a * (1 - top.a);
    if (alpha === 0) return { r: 0, g: 0, b: 0, a: 0 };
    const channel = (key) => (top[key] * top.a + bottom[key] * bottom.a * (1 - top.a)) / alpha;
    return { r: channel("r"), g: channel("g"), b: channel("b"), a: alpha };
  }

  function luminance({ r, g, b }) {
    const linear = (channel) => {
      const srgb = channel / 255;
      return srgb <= 0.04045 ? srgb / 12.92 : ((srgb + 0.055) / 1.055) ** 2.4;
    };
    return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
  }

  function contrast(foreground, background) {
    const a = luminance(foreground);
    const b = luminance(background);
    return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
  }

  function effectiveBackground(element) {
    const layers = [];
    let current = element;
    while (current) {
      const style = getComputedStyle(current);
      if (style.backgroundImage && style.backgroundImage !== "none") return { unknown: true, reason: "background-image", source: current };
      if (style.backdropFilter && style.backdropFilter !== "none") return { unknown: true, reason: "backdrop-filter", source: current };
      const color = parseColor(style.backgroundColor);
      if (color && color.a > 0) {
        layers.push(color);
        if (color.a >= 1) break;
      }
      current = current.parentElement;
    }
    let result = { r: 255, g: 255, b: 255, a: 1 };
    for (let index = layers.length - 1; index >= 0; index -= 1) result = blend(layers[index], result);
    return { color: result };
  }

  function isLargeText(style) {
    const size = parseFloat(style.fontSize);
    const weight = Number(style.fontWeight) || (style.fontWeight === "bold" ? 700 : 400);
    return size >= 24 || (size >= 18.66 && weight >= 700);
  }

  function accessibleName(element) {
    const labelled = element.getAttribute("aria-labelledby");
    if (labelled) {
      const text = labelled.split(/\s+/).map((id) => document.getElementById(id)?.textContent ?? "").join(" ").trim();
      if (text) return text;
    }
    const aria = element.getAttribute("aria-label");
    if (aria && aria.trim()) return aria.trim();
    if (element.labels && element.labels.length) {
      const text = [...element.labels].map((label) => label.textContent).join(" ").trim();
      if (text) return text;
    }
    if (["INPUT", "SELECT", "TEXTAREA"].includes(element.tagName)) {
      if (["submit", "button", "reset"].includes(element.type) && element.value) return element.value;
      return element.getAttribute("title")?.trim() ?? "";
    }
    const text = (element.innerText || element.textContent || "").trim();
    if (text) return text;
    const image = element.querySelector("img[alt]:not([alt=''])");
    if (image) return image.getAttribute("alt");
    const svgTitle = element.querySelector("svg title");
    if (svgTitle && svgTitle.textContent.trim()) return svgTitle.textContent.trim();
    return element.getAttribute("title")?.trim() ?? "";
  }

  function isInlineTextLink(element) {
    if (element.tagName !== "A") return false;
    const parent = element.parentElement;
    if (!parent || getComputedStyle(element).display !== "inline") return false;
    return ownTextNodes(parent).some((node) => node.textContent.trim().length > 20);
  }

  function createCollector(limit) {
    const issues = [];
    const counts = {};
    const totals = { error: 0, warn: 0, info: 0 };
    return {
      add(rule, severity, message, element, extra = {}) {
        counts[rule] = (counts[rule] ?? 0) + 1;
        totals[severity] += 1;
        if (counts[rule] > limit) return;
        const rect = element instanceof Element ? element.getBoundingClientRect() : null;
        issues.push({
          rule,
          severity,
          message,
          selector: element instanceof Element ? selectorFor(element) : String(element ?? ""),
          text: element instanceof Element ? textPreview(element) : "",
          rect: rect ? { x: round(rect.left + scrollX), y: round(rect.top + scrollY), w: round(rect.width), h: round(rect.height) } : null,
          ...extra,
        });
      },
      issues,
      counts,
      totals,
    };
  }

  function collectElements() {
    return [...document.body.querySelectorAll("*")].filter((element) => !SKIP_TAGS.has(element.tagName) && !element.closest("[aria-hidden='true'], [inert]") && isVisible(element));
  }

  function checkHorizontalOverflow(elements, report) {
    const viewportWidth = document.documentElement.clientWidth;
    const pageWidth = document.documentElement.scrollWidth;
    if (pageWidth <= viewportWidth + 1) return;
    const culprits = elements.filter((element) => {
      const rect = element.getBoundingClientRect();
      if (rect.right <= viewportWidth + 1 && rect.left >= -1) return false;
      const clipped = visibleRect(element, toPageRect(rect));
      if (!clipped) return false;
      return clipped.right - scrollX > viewportWidth + 1 || clipped.left - scrollX < -1;
    });
    const inFlow = culprits.filter((element) => !hasFixedAncestor(element));
    const outermost = (inFlow.length ? inFlow : culprits).filter((element) => !culprits.includes(element.parentElement));
    if (!outermost.length) {
      report.add("horizontal-overflow", "error", `Page scrolls horizontally: ${pageWidth}px content in ${viewportWidth}px viewport`, document.documentElement);
      return;
    }
    for (const element of outermost) {
      const rect = element.getBoundingClientRect();
      report.add("horizontal-overflow", "error", `Element pokes outside viewport (left ${round(rect.left)}, right ${round(rect.right)} of ${viewportWidth}px); page scrolls sideways`, element);
    }
  }

  function buildBoxes(elements) {
    const boxes = [];
    for (const element of elements) {
      const isControl = element.matches(CONTROL_SELECTOR);
      const isMedia = element.matches(MEDIA_SELECTOR);
      const ownTextRect = textRect(element);
      if (!isControl && !isMedia && !ownTextRect) continue;
      if (isMedia && element.closest(CONTROL_SELECTOR) && !isControl) continue;
      const ownBox = toPageRect(element.getBoundingClientRect());
      let baseRect = isControl || isMedia ? ownBox : ownTextRect;
      if (!isControl && !isMedia && clipsOverflow(getComputedStyle(element))) baseRect = intersectRects(baseRect, ownBox);
      const rect = baseRect && visibleRect(element, baseRect);
      if (!rect || rect.width < 1 || rect.height < 1) continue;
      boxes.push({
        element,
        rect,
        kind: isControl ? "control" : isMedia ? "media" : "text",
        floating: hasFixedAncestor(element),
      });
    }
    return boxes;
  }

  function overlapSeverity(first, second) {
    const kinds = [first.kind, second.kind].sort().join("+");
    if (kinds === "media+media") return null;
    if (kinds.includes("media")) return { severity: "warn", rule: "text-over-media", note: "content over media: confirm it is intended and readable (scrim / contrast)" };
    return { severity: "error", rule: "overlap", note: `${first.kind} overlaps ${second.kind}` };
  }

  function checkOverlaps(boxes, report, options) {
    const tolerance = options.overlapTolerance;
    const sorted = boxes.filter((box) => !box.floating).sort((a, b) => a.rect.top - b.rect.top);
    const mediaOverlapped = new Set();
    for (let first = 0; first < sorted.length; first += 1) {
      const a = sorted[first];
      for (let second = first + 1; second < sorted.length; second += 1) {
        const b = sorted[second];
        if (b.rect.top >= a.rect.bottom - tolerance) break;
        if (a.element.contains(b.element) || b.element.contains(a.element)) continue;
        const overlap = intersectRects(a.rect, b.rect);
        if (!overlap || overlap.width <= tolerance || overlap.height <= tolerance) continue;
        const smallerArea = Math.min(a.rect.width * a.rect.height, b.rect.width * b.rect.height);
        if (overlap.width * overlap.height < Math.max(16, smallerArea * 0.04)) continue;
        const verdict = overlapSeverity(a, b);
        if (!verdict) continue;
        if (verdict.rule === "text-over-media") {
          const textual = a.kind === "media" ? b : a;
          mediaOverlapped.add(textual.element);
        }
        report.add(verdict.rule, verdict.severity, `${verdict.note}; collides with ${selectorFor(b.element)} ("${textPreview(b.element)}") by ${round(overlap.width)}x${round(overlap.height)}px`, a.element);
      }
    }
    return mediaOverlapped;
  }

  function checkOcclusion(boxes, report) {
    const viewportWidth = innerWidth;
    const viewportHeight = innerHeight;
    for (const box of boxes) {
      if (box.floating || box.kind === "media") continue;
      const fullRect = box.element.getBoundingClientRect();
      if (fullRect.top < 0 || fullRect.bottom > viewportHeight) continue;
      const rect = intersectRects(fullRect, { left: 0, top: 0, right: viewportWidth, bottom: viewportHeight });
      if (!rect || rect.width < 8) continue;
      const inset = Math.min(4, rect.width / 4, rect.height / 4);
      const points = [
        [rect.left + rect.width / 2, rect.top + rect.height / 2],
        [rect.left + inset, rect.top + inset],
        [rect.right - inset, rect.top + inset],
        [rect.left + inset, rect.bottom - inset],
        [rect.right - inset, rect.bottom - inset],
      ];
      let blocker = null;
      let blocked = 0;
      for (const [x, y] of points) {
        const hit = document.elementFromPoint(x, y);
        if (!hit || hit === box.element || box.element.contains(hit) || hit.contains(box.element)) continue;
        const hitStyle = getComputedStyle(hit);
        if (hitStyle.pointerEvents === "none") continue;
        blocked += 1;
        blocker = hit;
      }
      if (blocked >= 3) report.add("occluded", "error", `Covered by ${selectorFor(blocker)} in the current viewport`, box.element);
    }
  }

  function checkClippedText(elements, report) {
    for (const element of elements) {
      if (!ownTextNodes(element).length && !element.matches("button, a, label, input, select")) continue;
      const style = getComputedStyle(element);
      const lineClamp = style.webkitLineClamp && style.webkitLineClamp !== "none";
      const ellipsis = style.textOverflow === "ellipsis";
      const horizontal = ["hidden", "clip"].includes(style.overflowX) && element.scrollWidth > element.clientWidth + 1;
      const vertical = ["hidden", "clip"].includes(style.overflowY) && element.scrollHeight > element.clientHeight + 1;
      if (!horizontal && !vertical) continue;
      if (ellipsis || lineClamp) {
        const severity = element.matches(CONTROL_SELECTOR) ? "warn" : "info";
        report.add("truncated-text", severity, "Text is truncated (ellipsis/line-clamp); make sure full text is reachable (title, tooltip, detail view)", element);
        continue;
      }
      const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.2;
      const visibleLines = element.clientHeight / lineHeight;
      const cleanLineCut = !horizontal && Math.abs(visibleLines - Math.round(visibleLines)) < 0.15;
      const detail = horizontal ? `width ${element.scrollWidth}>${element.clientWidth}` : `height ${element.scrollHeight}>${element.clientHeight}`;
      if (cleanLineCut) {
        report.add("truncated-text", "warn", `Text cut after ${Math.round(visibleLines)} line(s) with no ellipsis or fade (${detail}); signal the truncation`, element);
      } else {
        report.add("clipped-text", "error", `Text is cut off by overflow (${detail})`, element);
      }
    }
  }

  function checkContrast(elements, report, mediaOverlapped) {
    const seen = new Set();
    for (const element of elements) {
      if (!ownTextNodes(element).length) continue;
      const style = getComputedStyle(element);
      const color = parseColor(style.color);
      if (!color || color.a === 0) continue;
      const background = effectiveBackground(element);
      if (background.unknown || mediaOverlapped.has(element)) {
        const key = background.source ? selectorFor(background.source) : selectorFor(element);
        if (seen.has(key)) continue;
        seen.add(key);
        report.add("contrast-manual", "info", `Text sits on ${background.reason ?? "media"}; verify contrast on screenshot`, element);
        continue;
      }
      const ratio = contrast(blend(color, background.color), background.color);
      const required = isLargeText(style) ? 3 : 4.5;
      if (ratio + 0.01 < required) {
        const disabled = element.closest(":disabled, [aria-disabled='true']");
        report.add("contrast", disabled ? "info" : "error", `Contrast ${ratio.toFixed(2)}:1, needs ${required}:1 (${style.fontSize}, weight ${style.fontWeight})`, element, { ratio: round(ratio) });
      }
    }
  }

  function hasSpacingException(rect, others, minimum) {
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    const radius = minimum / 2;
    return !others.some((other) => {
      if (other.rect === rect) return false;
      const nearestX = Math.max(other.rect.left, Math.min(centerX, other.rect.right));
      const nearestY = Math.max(other.rect.top, Math.min(centerY, other.rect.bottom));
      return (nearestX - centerX) ** 2 + (nearestY - centerY) ** 2 < radius ** 2;
    });
  }

  function checkTouchTargets(elements, report, options) {
    const isNarrow = innerWidth < 768;
    const targets = elements
      .filter((element) => element.matches(CONTROL_SELECTOR) && !element.matches("input[type=hidden]"))
      .map((element) => ({ element, rect: element.getBoundingClientRect() }));
    for (const { element, rect } of targets) {
      if (isInlineTextLink(element)) continue;
      const smallest = Math.min(rect.width, rect.height);
      if (smallest < options.minTouchTarget) {
        const spaced = hasSpacingException(rect, targets.filter((target) => !target.element.contains(element) && !element.contains(target.element)), options.minTouchTarget);
        if (spaced) report.add("touch-target-comfort", "warn", `Target ${round(rect.width)}x${round(rect.height)}px passes WCAG 2.5.8 only by spacing; enlarge the hit area`, element);
        else report.add("touch-target", "error", `Target ${round(rect.width)}x${round(rect.height)}px is below ${options.minTouchTarget}px and crowded by neighbours (WCAG 2.5.8)`, element);
      } else if (isNarrow && smallest < options.comfortableTouchTarget) {
        report.add("touch-target-comfort", "warn", `Target ${round(rect.width)}x${round(rect.height)}px is below ${options.comfortableTouchTarget}px on a touch-size viewport`, element);
      }
    }
  }

  function checkNames(elements, report) {
    for (const element of elements) {
      if (!element.matches(CONTROL_SELECTOR)) continue;
      if (!accessibleName(element)) report.add("accessible-name", "error", "Interactive element has no accessible name", element);
      if (element.matches("input, textarea") && !element.labels?.length && !element.getAttribute("aria-label") && element.placeholder) {
        report.add("placeholder-label", "warn", "Placeholder used as the only label; it disappears on input", element);
      }
      if (element.tagName === "A") {
        const href = element.getAttribute("href") ?? "";
        if (href === "#" || href.startsWith("javascript:")) report.add("dead-link", "warn", `Link goes nowhere (href="${href}"); use a button or a real URL`, element);
      }
    }
  }

  function checkImages(report) {
    for (const image of document.images) {
      if (!isVisible(image)) continue;
      if (!image.hasAttribute("alt")) report.add("img-alt", "error", "Image has no alt attribute (use alt=\"\" if decorative)", image);
      if (image.complete && image.naturalWidth === 0) {
        report.add("img-broken", "error", `Image failed to load: ${image.currentSrc || image.src}`, image);
        continue;
      }
      const style = getComputedStyle(image);
      const hasDimensions = image.hasAttribute("width") && image.hasAttribute("height");
      if (!hasDimensions && style.aspectRatio === "auto") report.add("img-dimensions", "warn", "No width/height or aspect-ratio: layout shifts while loading", image);
      if (!image.naturalWidth) continue;
      const rect = image.getBoundingClientRect();
      const naturalRatio = image.naturalWidth / image.naturalHeight;
      const renderedRatio = rect.width / rect.height;
      if (style.objectFit === "fill" && Math.abs(renderedRatio - naturalRatio) / naturalRatio > 0.03) {
        report.add("img-distorted", "error", `Image is stretched: rendered ratio ${renderedRatio.toFixed(2)} vs natural ${naturalRatio.toFixed(2)} (use object-fit: cover/contain)`, image);
      }
      if (rect.width * devicePixelRatio > image.naturalWidth * 1.25 && !image.currentSrc.endsWith(".svg")) {
        report.add("img-upscaled", "warn", `Image upscaled: ${image.naturalWidth}px source shown at ${round(rect.width * devicePixelRatio)} device px (blurry)`, image);
      }
    }
  }

  function checkHeadings(report) {
    const headings = [...document.querySelectorAll(HEADING_SELECTOR)].filter(isVisible);
    const h1Count = headings.filter((heading) => heading.tagName === "H1").length;
    if (h1Count === 0) report.add("heading-structure", "warn", "Page has no visible h1", document.body);
    if (h1Count > 1) report.add("heading-structure", "warn", `Page has ${h1Count} visible h1 elements`, headings.find((heading) => heading.tagName === "H1"));
    let previous = 0;
    for (const heading of headings) {
      const level = Number(heading.tagName[1]);
      if (previous && level > previous + 1) report.add("heading-structure", "warn", `Heading level jumps from h${previous} to h${level}`, heading);
      previous = level;
    }
  }

  function wordRects(textNode) {
    const range = document.createRange();
    const text = textNode.textContent;
    const result = [];
    const pattern = /\S+/g;
    let match;
    while ((match = pattern.exec(text))) {
      range.setStart(textNode, match.index);
      range.setEnd(textNode, match.index + match[0].length);
      const rect = range.getBoundingClientRect();
      if (rect.width > 0) result.push({ word: match[0], top: rect.top });
    }
    return result;
  }

  function checkTypography(elements, report) {
    for (const heading of document.querySelectorAll(HEADING_SELECTOR)) {
      if (!isVisible(heading)) continue;
      const words = ownTextNodes(heading).flatMap(wordRects);
      if (words.length < 4) continue;
      const last = words[words.length - 1];
      const beforeLast = words[words.length - 2];
      if (last.top - beforeLast.top > 2 && last.word.length < 12) {
        report.add("heading-widow", "warn", `Last word "${last.word}" sits alone on its line; use text-wrap: balance or rephrase`, heading);
      }
    }
    for (const element of elements) {
      const style = getComputedStyle(element);
      const fontSize = parseFloat(style.fontSize);
      if (element.matches(TEXT_BLOCK_SELECTOR)) {
        const length = (element.textContent || "").trim().length;
        const charsPerLine = element.getBoundingClientRect().width / (fontSize * 0.5);
        if (length > 160 && charsPerLine > 90) report.add("line-length", "warn", `About ${Math.round(charsPerLine)} characters per line; aim for 45-80`, element);
      }
      if (ownTextNodes(element).length && fontSize < 12) report.add("tiny-text", "warn", `Font size ${fontSize}px is too small to read comfortably`, element);
      if (innerWidth < 768 && element.matches("input:not([type=checkbox]):not([type=radio]), select, textarea") && fontSize < 16) {
        report.add("input-zoom", "warn", `Input font ${fontSize}px < 16px triggers zoom on iOS`, element);
      }
      if (ownTextNodes(element).length && parseFloat(style.lineHeight) && parseFloat(style.lineHeight) < fontSize * 1.05 && element.getClientRects().length && textRect(element)?.height > fontSize * 2.2) {
        report.add("tight-leading", "warn", `Multi-line text with line-height ${style.lineHeight} at ${fontSize}px: lines may touch`, element);
      }
    }
  }

  function checkDuplicateIds(report) {
    const seen = new Map();
    for (const element of document.querySelectorAll("[id]")) {
      seen.set(element.id, (seen.get(element.id) ?? 0) + 1);
    }
    for (const [id, count] of seen) {
      if (count > 1) report.add("duplicate-id", "warn", `id "${id}" is used ${count} times`, document.getElementById(id));
    }
  }

  function checkFloatingCoverage(elements, report) {
    const viewportArea = innerWidth * innerHeight;
    let covered = 0;
    const isPinned = (element) => {
      const style = getComputedStyle(element);
      if (style.position === "fixed") return true;
      if (style.position !== "sticky") return false;
      const offset = parseFloat(style.top);
      return Number.isFinite(offset) && Math.abs(element.getBoundingClientRect().top - offset) <= 1;
    };
    const pinned = elements.filter(isPinned);
    const floating = pinned.filter((element) => !pinned.some((other) => other !== element && other.contains(element)));
    for (const element of floating) {
      const rect = element.getBoundingClientRect();
      const visible = intersectRects({ left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom }, { left: 0, top: 0, right: innerWidth, bottom: innerHeight });
      if (visible) covered += visible.width * visible.height;
    }
    const share = covered / viewportArea;
    if (share > 0.3) report.add("floating-coverage", "warn", `Fixed/sticky layers cover ${Math.round(share * 100)}% of the viewport`, floating[0]);
  }

  function checkAlignment(elements, report) {
    const edges = [];
    for (const element of elements) {
      if (!element.matches(`${HEADING_SELECTOR}, ${TEXT_BLOCK_SELECTOR}, img, button`)) continue;
      const rect = element.getBoundingClientRect();
      if (rect.width < 40) continue;
      edges.push({ element, left: rect.left + scrollX, top: rect.top + scrollY });
    }
    edges.sort((a, b) => a.top - b.top);
    for (let index = 1; index < edges.length; index += 1) {
      const current = edges[index];
      const previous = edges[index - 1];
      const delta = Math.abs(current.left - previous.left);
      if (delta > 0.5 && delta <= 3 && current.top - previous.top < 400) {
        report.add("alignment-near-miss", "info", `Left edge is ${round(delta)}px off ${selectorFor(previous.element)}; align to the grid or offset deliberately`, current.element);
      }
    }
  }

  function tokenReport(elements) {
    const tally = (map, key) => map.set(key, (map.get(key) ?? 0) + 1);
    const fontSizes = new Map();
    const families = new Map();
    const colors = new Map();
    const radii = new Map();
    const shadows = new Map();
    const spacing = new Map();
    for (const element of elements) {
      const style = getComputedStyle(element);
      if (ownTextNodes(element).length) {
        tally(fontSizes, style.fontSize);
        tally(families, style.fontFamily.split(",")[0].replace(/["']/g, "").trim());
        tally(colors, style.color);
      }
      const background = parseColor(style.backgroundColor);
      if (background && background.a > 0) tally(colors, style.backgroundColor);
      if (style.borderRadius !== "0px") tally(radii, style.borderRadius);
      if (style.boxShadow !== "none") tally(shadows, style.boxShadow);
      for (const property of ["marginTop", "marginBottom", "paddingTop", "paddingBottom", "paddingLeft", "paddingRight", "rowGap", "columnGap"]) {
        const value = parseFloat(style[property]);
        if (value > 0) tally(spacing, `${round(value)}px`);
      }
    }
    const top = (map, limit = 12) => [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, limit).map(([value, count]) => `${value} ×${count}`);
    const offGrid = [...spacing.keys()].filter((value) => parseFloat(value) % 4 !== 0).length;
    return {
      fontSizes: { distinct: fontSizes.size, top: top(fontSizes) },
      fontFamilies: { distinct: families.size, top: top(families) },
      colors: { distinct: colors.size, top: top(colors) },
      radii: { distinct: radii.size, top: top(radii) },
      shadows: { distinct: shadows.size, top: top(shadows, 5) },
      spacing: { distinct: spacing.size, offFourPxGrid: offGrid, top: top(spacing) },
    };
  }

  function checkTokenSprawl(tokens, report) {
    if (tokens.fontSizes.distinct > 10) report.add("token-sprawl", "warn", `${tokens.fontSizes.distinct} distinct font sizes; consolidate into a type scale (6-8 steps)`, document.body);
    if (tokens.fontFamilies.distinct > 3) report.add("token-sprawl", "warn", `${tokens.fontFamilies.distinct} font families in use: ${tokens.fontFamilies.top.join(", ")}`, document.body);
    if (tokens.radii.distinct > 5) report.add("token-sprawl", "info", `${tokens.radii.distinct} distinct border radii; keep a radius scale`, document.body);
    if (tokens.shadows.distinct > 4) report.add("token-sprawl", "info", `${tokens.shadows.distinct} distinct shadows; keep 2-3 elevation levels`, document.body);
  }

  function summarize(report) {
    return { total: report.totals, rules: report.counts, listed: report.issues.length, verdict: report.totals.error === 0 ? "pass" : "fail" };
  }

  function audit(userOptions = {}) {
    const options = { ...defaults, ...userOptions };
    const report = createCollector(options.maxIssuesPerRule);
    const elements = collectElements();
    checkHorizontalOverflow(elements, report);
    const boxes = buildBoxes(elements);
    const mediaOverlapped = checkOverlaps(boxes, report, options);
    checkOcclusion(boxes, report);
    checkClippedText(elements, report);
    checkContrast(elements, report, mediaOverlapped);
    checkTouchTargets(elements, report, options);
    checkNames(elements, report);
    checkImages(report);
    checkHeadings(report);
    checkTypography(elements, report);
    checkDuplicateIds(report);
    checkFloatingCoverage(elements, report);
    checkAlignment(elements, report);
    const tokens = options.includeTokenReport ? tokenReport(elements) : null;
    if (tokens) checkTokenSprawl(tokens, report);
    const order = { error: 0, warn: 1, info: 2 };
    report.issues.sort((a, b) => order[a.severity] - order[b.severity]);
    return {
      url: location.href,
      viewport: { width: innerWidth, height: innerHeight, dpr: devicePixelRatio, scrollY },
      document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight },
      elementsChecked: elements.length,
      summary: summarize(report),
      issues: report.issues,
      tokens,
    };
  }

  window.__designAudit = audit;
  return "window.__designAudit(options) is ready";
})();
