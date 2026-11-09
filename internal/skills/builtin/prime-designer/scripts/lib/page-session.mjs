const METRICS_INIT_SCRIPT = `
  window.__designMetrics = { cls: 0, shifts: [], lcp: 0 };
  try {
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.hadRecentInput) continue;
        window.__designMetrics.cls += entry.value;
        const source = entry.sources && entry.sources[0] && entry.sources[0].node;
        window.__designMetrics.shifts.push({
          value: Math.round(entry.value * 1000) / 1000,
          time: Math.round(entry.startTime),
          node: source && source.nodeType === 1 ? source.tagName.toLowerCase() + (source.id ? "#" + source.id : "") + (source.className && typeof source.className === "string" ? "." + source.className.trim().split(/\\s+/).slice(0, 2).join(".") : "") : null,
        });
      }
    }).observe({ type: "layout-shift", buffered: true });
    new PerformanceObserver((list) => {
      const entries = list.getEntries();
      window.__designMetrics.lcp = Math.round(entries[entries.length - 1].startTime);
    }).observe({ type: "largest-contentful-paint", buffered: true });
  } catch (error) {}
`;

export function attachRuntimeListeners(page) {
  const runtime = { consoleErrors: [], pageErrors: [], failedRequests: [] };
  page.on("console", (message) => {
    if (message.type() === "error") runtime.consoleErrors.push(message.text().slice(0, 300));
  });
  page.on("pageerror", (error) => runtime.pageErrors.push(String(error.message).slice(0, 300)));
  page.on("requestfailed", (request) => {
    runtime.failedRequests.push({ url: request.url(), reason: request.failure()?.errorText ?? "failed" });
  });
  page.on("response", (response) => {
    if (response.status() >= 400) runtime.failedRequests.push({ url: response.url(), reason: `HTTP ${response.status()}` });
  });
  return runtime;
}

export async function preparePage(page) {
  await page.addInitScript(METRICS_INIT_SCRIPT);
}

export async function loadAndSettle(page, options) {
  await page.goto(options.target, { waitUntil: "load", timeout: 45000 });
  await page.waitForLoadState("networkidle", { timeout: 10000 }).catch(() => {});
  await page.evaluate(() => document.fonts?.ready);
  if (options.scroll) await scrollThrough(page);
  await page.waitForTimeout(options.wait);
}

async function scrollThrough(page) {
  await page.evaluate(async () => {
    const pause = (ms) => new Promise((done) => setTimeout(done, ms));
    const step = Math.max(200, Math.floor(innerHeight * 0.8));
    for (let y = 0; y < document.documentElement.scrollHeight; y += step) {
      scrollTo(0, y);
      await pause(120);
    }
    scrollTo(0, document.documentElement.scrollHeight);
    await pause(250);
    scrollTo(0, 0);
    await pause(300);
  });
  await page.waitForLoadState("networkidle", { timeout: 5000 }).catch(() => {});
}

export async function readMetrics(page) {
  return page.evaluate(() => {
    const metrics = window.__designMetrics ?? { cls: 0, shifts: [], lcp: 0 };
    return {
      cls: Math.round(metrics.cls * 1000) / 1000,
      lcpMs: metrics.lcp,
      worstShifts: [...metrics.shifts].sort((a, b) => b.value - a.value).slice(0, 5),
    };
  });
}
