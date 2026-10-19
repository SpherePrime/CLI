export function speakableText(markdown: string, codeLabel: string): string {
  return markdown
    .replace(/```[\s\S]*?```/g, ` ${codeLabel}. `)
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/https?:\/\/\S+/g, '')
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '')
    .replace(/^\s*\d+[.)]\s+/gm, '')
    .replace(/^\s*>\s?/gm, '')
    .replace(/\|/g, ' ')
    .replace(/[*_~]{1,3}([^*_~]+)[*_~]{1,3}/g, '$1')
    .replace(/<[^>]+>/g, '')
    .replace(/[ \t]+/g, ' ')
    .replace(/\n{2,}/g, '\n')
    .trim()
}

function pieces(sentence: string, limit: number): string[] {
  if (sentence.length <= limit) return [sentence]
  const parts: string[] = []
  let current = ''
  for (const word of sentence.split(' ')) {
    if (current && current.length + word.length + 1 > limit) {
      parts.push(current)
      current = word
    } else current = current ? `${current} ${word}` : word
  }
  if (current) parts.push(current)
  return parts
}

export function speechChunks(text: string, limit = 220): string[] {
  const chunks: string[] = []
  let current = ''
  for (const sentence of text.split(/(?<=[.!?…])\s+|\n+/)) {
    for (const piece of pieces(sentence.trim(), limit)) {
      if (!piece) continue
      if (current && current.length + piece.length + 1 > limit) {
        chunks.push(current)
        current = piece
      } else current = current ? `${current} ${piece}` : piece
    }
  }
  if (current) chunks.push(current)
  return chunks.slice(0, 40)
}
