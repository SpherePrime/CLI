import { RGBA } from "@opentui/core";

export const theme = {
  primary: RGBA.fromHex("#fab283"),
  secondary: RGBA.fromHex("#5c9cf5"),
  accent: RGBA.fromHex("#9d7cd8"),
  error: RGBA.fromHex("#e06c75"),
  warning: RGBA.fromHex("#f5a742"),
  success: RGBA.fromHex("#7fd88f"),
  info: RGBA.fromHex("#56b6c2"),
  text: RGBA.fromHex("#eeeeee"),
  textMuted: RGBA.fromHex("#808080"),
  background: RGBA.fromHex("#0a0a0a"),
  backgroundPanel: RGBA.fromHex("#141414"),
  backgroundElement: RGBA.fromHex("#1e1e1e"),
  backgroundMenu: RGBA.fromHex("#141414"),
  border: RGBA.fromHex("#484848"),
  borderActive: RGBA.fromHex("#606060"),
  borderSubtle: RGBA.fromHex("#3c3c3c"),
} as const;

export type Theme = typeof theme;

export function selectedForeground(t: Theme): RGBA {
  return t.primary;
}

export function tint(base: RGBA, target: RGBA, alpha: number): RGBA {
  const r = Math.round(base.r * (1 - alpha) + target.r * alpha);
  const g = Math.round(base.g * (1 - alpha) + target.g * alpha);
  const b = Math.round(base.b * (1 - alpha) + target.b * alpha);
  return RGBA.fromValues(r, g, b, base.a ?? 255);
}