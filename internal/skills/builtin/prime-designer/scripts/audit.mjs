#!/usr/bin/env node
import { mkdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs, USAGE } from "./lib/args.mjs";
import { attachRuntimeListeners, preparePage, loadAndSettle, readMetrics } from "./lib/page-session.mjs";
import { walkFocus } from "./lib/focus-walk.mjs";
import { runtimeIssues, countBySeverity, writeReports } from "./lib/report.mjs";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const auditSource = readFileSync(join(scriptDir, "layout-audit.js"), "utf8");

async function loadPlaywright() {
  try {
    return await import("playwright");
  } catch {
    console.error("Playwright is missing. Run once inside scripts/:  npm install && npx playwright install chromium");
    process.exit(2);
  }
}

async function launchBrowser(chromium, headed) {
  const attempts = [{}, { channel: "chrome" }, { channel: "msedge" }];
  for (const attempt of attempts) {
    try {
      return await chromium.launch({ headless: !headed, ...attempt });
    } catch (error) {
      if (attempt.channel === "msedge") throw new Error(`No browser available. Run: npx playwright install chromium\n${error.message}`);
    }
  }
  return null;
}

async function auditRun(browser, options, viewport, scheme) {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: viewport.isMobile ? 2 : 1,
    isMobile: Boolean(viewport.isMobile),
    hasTouch: Boolean(viewport.hasTouch),
    colorScheme: scheme,
    reducedMotion: options.reducedMotion ? "reduce" : "no-preference",
  });
  const page = await context.newPage();
  const runtime = attachRuntimeListeners(page);
  await preparePage(page);
  await loadAndSettle(page, options);

  const label = `${viewport.name}-${scheme}`;
  const screenshots = { viewport: `${label}-first-screen.png`, full: `${label}-full.png` };
  await page.screenshot({ path: join(options.out, screenshots.viewport) });
  await page.screenshot({ path: join(options.out, screenshots.full), fullPage: true });

  await page.evaluate(auditSource);
  const layout = await page.evaluate(() => window.__designAudit());
  const metrics = await readMetrics(page);
  const focus = await walkFocus(page, options.tabStops);
  await context.close();

  const extraIssues = [...focus.issues, ...runtimeIssues(runtime, metrics)];
  const issues = [...layout.issues, ...extraIssues];
  const totals = countBySeverity(extraIssues);
  totals.error += layout.summary.total.error;
  totals.warn += layout.summary.total.warn;
  totals.info += layout.summary.total.info;
  return {
    name: viewport.name,
    width: viewport.width,
    height: viewport.height,
    scheme,
    reducedMotion: options.reducedMotion,
    totals,
    metrics,
    layout: { document: layout.document, rules: layout.summary.rules, tokens: layout.tokens },
    focusOrder: focus.order,
    issues,
    screenshots,
  };
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (!options) {
    console.log(USAGE);
    process.exit(1);
  }
  options.out = resolve(options.out);
  mkdirSync(options.out, { recursive: true });
  const { chromium } = await loadPlaywright();
  const browser = await launchBrowser(chromium, options.headed);
  const runs = [];
  try {
    for (const scheme of options.schemes) {
      for (const viewport of options.viewports) {
        const run = await auditRun(browser, options, viewport, scheme);
        runs.push(run);
        console.log(`${run.name.padEnd(14)} ${scheme.padEnd(5)} errors ${String(run.totals.error).padStart(3)}  warnings ${String(run.totals.warn).padStart(3)}  CLS ${run.metrics.cls}`);
      }
    }
  } finally {
    await browser.close();
  }
  const result = writeReports(options.out, options.target, runs);
  console.log(`\n${result.verdict}: ${result.errorCount} errors, ${result.warnCount} warnings`);
  console.log(`Report: ${join(options.out, "report.md")}`);
  process.exit(result.verdict === "PASS" ? 0 : 1);
}

main().catch((error) => {
  console.error(error);
  process.exit(2);
});
