import { RGBA } from "@opentui/core";

export type SpinnerColor = { r: number; g: number; b: number; a: number };

function colorString(color: RGBA | SpinnerColor): string {
  if (color instanceof RGBA) return color.toString()
  return `\u001b[38;2;${color.r};${color.g};${color.b}m`
}

export function createFrames(options: { color: RGBA; style: "blocks" | "dots"; inactiveFactor?: number }) {
  const frames = options.style === "blocks" ? ["▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂"] : ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]
  return frames.map((f) => `\u001b[0m${colorString(options.color)}${f}\u001b[0m`)
}