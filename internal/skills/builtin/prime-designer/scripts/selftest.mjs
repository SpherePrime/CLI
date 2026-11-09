#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { readFileSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const outRoot = mkdtempSync(join(tmpdir(), "prime-designer-selftest-"));

const EXPECTED_IN_DEFECTS = [
  "horizontal-overflow",
  "overlap",
  "occluded",
  "clipped-text",
  "contrast",
  "touch-target",
  "accessible-name",
  "img-alt",
  "img-distorted",
  "img-dimensions",
  "heading-structure",
  "heading-widow",
  "placeholder-label",
  "dead-link",
  "line-length",
  "focus-indicator",
];

function runAudit(fixture, viewports) {
  const out = join(outRoot, fixture.replace(".html", ""));
  try {
    execFileSync(process.execPath, [join(scriptDir, "audit.mjs"), join(scriptDir, "fixtures", fixture), "--out", out, "--viewports", viewports], { stdio: "pipe" });
  } catch (error) {
    if (error.status === 2) throw new Error(error.stderr.toString());
  }
  return JSON.parse(readFileSync(join(out, "report.json"), "utf8"));
}

const failures = [];

const clean = runAudit("clean.html", "320x640,390x844,1440x900");
if (clean.errorCount !== 0 || clean.warnCount !== 0) {
  const rules = clean.runs.flatMap((run) => run.issues.filter((issue) => issue.severity !== "info").map((issue) => `${run.name}:${issue.rule}`));
  failures.push(`clean.html should pass without warnings, got: ${rules.join(", ")}`);
}

const defects = runAudit("defects.html", "390x844,1440x900");
const found = new Set(defects.runs.flatMap((run) => run.issues.map((issue) => issue.rule)));
for (const rule of EXPECTED_IN_DEFECTS) {
  if (!found.has(rule)) failures.push(`defects.html should trigger "${rule}"`);
}

if (failures.length) {
  console.error("SELFTEST FAILED");
  for (const failure of failures) console.error(`  - ${failure}`);
  process.exit(1);
}
console.log(`SELFTEST PASSED: clean fixture clean, ${EXPECTED_IN_DEFECTS.length} defect rules detected`);
