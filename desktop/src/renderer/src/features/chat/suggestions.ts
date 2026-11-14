export type FileSuggestion = {
  label: string
  description: string
  file: string
}

export function nextSuggestionIndex(
  current: number,
  length: number,
  key: 'ArrowUp' | 'ArrowDown'
) {
  if (length <= 0) return 0
  const step = key === 'ArrowDown' ? 1 : -1
  return (current + step + length) % length
}

export function applyFileSuggestion(text: string, file: string) {
  return text.replace(/@[^\s]*$/, `@${file} `)
}
