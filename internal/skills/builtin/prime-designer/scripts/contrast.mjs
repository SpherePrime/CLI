#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { parseHex, contrastRatio, wcagGrade } from "./lib/color.mjs";

const HEX_IN_TEXT = /#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b/g;

function collectColors(args) {
  const themeIndex = args.indexOf("--theme");
  if (themeIndex !== -1) {
    const text = readFileSync(args[themeIndex + 1], "utf8");
    return [...new Set(text.match(HEX_IN_TEXT) ?? [])];
  }
  return args.filter((arg) => !arg.startsWith("--"));
}

function printUsage() {
  console.log("Usage:");
  console.log("  node contrast.mjs <hex> <hex> [hex...]");
  console.log("  node contrast.mjs --theme ../assets/themes/ocean-depths.md");
  console.log("Prints every foreground/background pair with WCAG ratio and grade.");
}

const colors = collectColors(process.argv.slice(2));

if (colors.length < 2) {
  printUsage();
  process.exit(colors.length === 0 ? 0 : 1);
}

const rows = [];
for (let first = 0; first < colors.length; first += 1) {
  for (let second = first + 1; second < colors.length; second += 1) {
    const ratio = contrastRatio(parseHex(colors[first]), parseHex(colors[second]));
    rows.push({ pair: `${colors[first]} / ${colors[second]}`, ratio, grade: wcagGrade(ratio) });
  }
}

rows.sort((a, b) => b.ratio - a.ratio);
console.log("| Pair | Ratio | Grade |");
console.log("| --- | ---: | --- |");
for (const row of rows) {
  console.log(`| ${row.pair} | ${row.ratio.toFixed(2)}:1 | ${row.grade} |`);
}
