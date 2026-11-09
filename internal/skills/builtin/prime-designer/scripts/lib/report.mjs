import { writeFileSync } from "node:fs";
import { join } from "node:path";

const CLS_LIMIT = 0.1;
const SEVERITY_ICON = { error: "✖", warn: "▲", info: "·" };

export function runtimeIssues(runtime, metrics) {
  const issues = [];
  for (const message of runtime.pageErrors) issues.push({ rule: "js-exception", severity: "error", message });
  for (const message of runtime.consoleErrors) issues.push({ rule: "console-error", severity: "error", message });
  for (const request of runtime.failedRequests) issues.push({ rule: "failed-request", severity: "error", message: `${request.reason}: ${request.url}` });
  if (metrics.cls > CLS_LIMIT) {
    const culprits = metrics.worstShifts.map((shift) => `${shift.node ?? "?"} (${shift.value})`).join(", ");
    issues.push({ rule: "layout-shift", severity: "error", message: `CLS ${metrics.cls} > ${CLS_LIMIT}. Worst: ${culprits}` });
  }
  return issues;
}

export function countBySeverity(issues) {
  const totals = { error: 0, warn: 0, info: 0 };
  for (const issue of issues) totals[issue.severity] += 1;
  return totals;
}

function issueLine(issue) {
  const where = issue.selector ? ` \`${issue.selector}\`` : "";
  const text = issue.text ? ` “${issue.text}”` : "";
  return `- ${SEVERITY_ICON[issue.severity]} **${issue.rule}**${where}${text}: ${issue.message}`;
}

function runSection(run) {
  const lines = [];
  const totals = run.totals;
  lines.push(`## ${run.name} · ${run.width}×${run.height} · ${run.scheme}${run.reducedMotion ? " · reduced motion" : ""}`);
  lines.push("");
  lines.push(`Errors ${totals.error} · warnings ${totals.warn} · notes ${totals.info} · CLS ${run.metrics.cls} · LCP ${run.metrics.lcpMs || "n/a"} ms · page ${run.layout.document.width}×${run.layout.document.height}`);
  lines.push("");
  lines.push(`Screenshots: [first screen](${run.screenshots.viewport}) · [full page](${run.screenshots.full})`);
  lines.push("");
  const issues = run.issues.filter((issue) => issue.severity !== "info");
  const notes = run.issues.filter((issue) => issue.severity === "info");
  if (!issues.length) lines.push("No blocking or warning issues detected by automation.");
  for (const issue of issues) lines.push(issueLine(issue));
  if (notes.length) {
    lines.push("");
    lines.push("<details><summary>Notes for manual review</summary>");
    lines.push("");
    for (const issue of notes) lines.push(issueLine(issue));
    lines.push("");
    lines.push("</details>");
  }
  if (run.focusOrder.length) {
    lines.push("");
    lines.push(`<details><summary>Keyboard order (${run.focusOrder.length} stops)</summary>`);
    lines.push("");
    run.focusOrder.forEach((stop, index) => lines.push(`${index + 1}. ${stop}`));
    lines.push("");
    lines.push("</details>");
  }
  lines.push("");
  return lines.join("\n");
}

function tokenSection(run) {
  if (!run?.layout.tokens) return "";
  const tokens = run.layout.tokens;
  const row = (label, entry) => `| ${label} | ${entry.distinct} | ${entry.top.slice(0, 6).join(", ")} |`;
  return [
    `## Design tokens in use (${run.name})`,
    "",
    "| Token | Distinct | Most used |",
    "| --- | ---: | --- |",
    row("Font sizes", tokens.fontSizes),
    row("Font families", tokens.fontFamilies),
    row("Colors", tokens.colors),
    row("Radii", tokens.radii),
    row("Shadows", tokens.shadows),
    `| Spacing | ${tokens.spacing.distinct} (${tokens.spacing.offFourPxGrid} off 4px grid) | ${tokens.spacing.top.slice(0, 6).join(", ")} |`,
    "",
  ].join("\n");
}

export function writeReports(outDir, target, runs) {
  const errorCount = runs.reduce((sum, run) => sum + run.totals.error, 0);
  const warnCount = runs.reduce((sum, run) => sum + run.totals.warn, 0);
  const verdict = errorCount === 0 ? "PASS" : "FAIL";
  const header = [
    `# Visual QA report — ${verdict}`,
    "",
    `Target: ${target}`,
    `Generated: ${new Date().toISOString()}`,
    "",
    "Automation catches mechanical defects only. Still open every screenshot and judge hierarchy, rhythm, and intent by eye (see references/quality-gate.md).",
    "",
    "| Run | Errors | Warnings | CLS | Horizontal scroll |",
    "| --- | ---: | ---: | ---: | --- |",
    ...runs.map((run) => `| ${run.name} ${run.scheme} | ${run.totals.error} | ${run.totals.warn} | ${run.metrics.cls} | ${run.layout.document.width > run.width ? "yes" : "no"} |`),
    "",
  ].join("\n");
  const body = runs.map(runSection).join("\n");
  const markdown = `${header}\n${tokenSection(runs.find((run) => run.width >= 1280) ?? runs[0])}\n${body}`;
  writeFileSync(join(outDir, "report.md"), markdown, "utf8");
  writeFileSync(join(outDir, "report.json"), JSON.stringify({ target, verdict, errorCount, warnCount, runs }, null, 2), "utf8");
  return { verdict, errorCount, warnCount };
}
