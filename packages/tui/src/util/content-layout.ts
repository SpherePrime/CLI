export type ContentLayout = { maxContent: number; sidePad: number }

export function contentLayout(width: number): ContentLayout {
  const maxContent = Math.max(0, Math.min(width - 4, 120))
  const sidePad = Math.max(2, Math.floor((width - maxContent) / 2))
  return { maxContent, sidePad }
}