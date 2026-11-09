import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parseViewports } from "./viewports.mjs";

export const USAGE = `Usage: node audit.mjs <url|file.html> [options]

Options:
  --out <dir>             Output folder for screenshots and reports (default: qa-report)
  --viewports <list>      Comma list like 375x812,1440x900 (default: 320, 390, 768, 1280, 1440, 1920)
  --dark                  Also run every viewport with prefers-color-scheme: dark
  --reduced-motion        Run with prefers-reduced-motion: reduce
  --tab-stops <n>         Keyboard focus stops to walk per viewport (default: 40)
  --wait <ms>             Extra settle time after load (default: 600)
  --no-scroll             Do not scroll through the page before auditing
  --headed                Show the browser window`;

function normalizeTarget(target) {
  if (/^https?:\/\//i.test(target) || target.startsWith("file:")) return target;
  const absolute = resolve(target);
  if (existsSync(absolute)) return pathToFileURL(absolute).href;
  return `http://${target}`;
}

export function parseArgs(argv) {
  const options = {
    target: null,
    out: "qa-report",
    viewports: parseViewports(null),
    schemes: ["light"],
    reducedMotion: false,
    tabStops: 40,
    wait: 600,
    scroll: true,
    headed: false,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    const next = () => argv[++index];
    if (arg === "--out") options.out = next();
    else if (arg === "--viewports") options.viewports = parseViewports(next());
    else if (arg === "--dark") options.schemes = ["light", "dark"];
    else if (arg === "--reduced-motion") options.reducedMotion = true;
    else if (arg === "--tab-stops") options.tabStops = Number(next());
    else if (arg === "--wait") options.wait = Number(next());
    else if (arg === "--no-scroll") options.scroll = false;
    else if (arg === "--headed") options.headed = true;
    else if (arg === "--help" || arg === "-h") return null;
    else if (!arg.startsWith("--")) options.target = normalizeTarget(arg);
    else throw new Error(`Unknown option ${arg}`);
  }
  return options.target ? options : null;
}
