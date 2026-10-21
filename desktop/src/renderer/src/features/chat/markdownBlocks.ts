const fencePattern = /^ {0,3}(`{3,}|~{3,})/
const closingFence = /^ {0,3}(`{3,}|~{3,})\s*$/
const blankLine = /^\s*$/
const attachedStart = /^(\s|[-*+](\s|$)|\d{1,9}[.)](\s|$))/
const unsplittable = /^ {0,3}\[[^\]]+\]:|<!--|^ {0,3}<(script|pre|style|textarea)\b/im

export function splitMarkdownBlocks(text: string): string[] {
  if (!text || unsplittable.test(text)) return [text]
  const blocks: string[] = []
  let current: string[] = []
  let blanks: string[] = []
  let fence: { marker: string; length: number } | null = null
  for (const line of text.split('\n')) {
    if (fence) {
      current.push(line)
      const closing = closingFence.exec(line)
      if (closing && closing[1][0] === fence.marker && closing[1].length >= fence.length)
        fence = null
      continue
    }
    const opening = fencePattern.exec(line)
    if (blankLine.test(line)) {
      blanks.push(line)
      continue
    }
    if (blanks.length && current.length && !attachedStart.test(line)) {
      blocks.push(current.join('\n'))
      current = []
    } else current.push(...blanks)
    blanks = []
    current.push(line)
    if (opening) fence = { marker: opening[1][0], length: opening[1].length }
  }
  current.push(...blanks)
  if (current.length || !blocks.length) blocks.push(current.join('\n'))
  return blocks
}
