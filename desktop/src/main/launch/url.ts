const ansi = /\u001b\[[0-9;?]*[ -/]*[@-~]|\u001b\][^\u0007]*(\u0007|\u001b\\)/g

export function stripAnsi(text: string): string {
  return text.replace(ansi, '')
}

export function detectLocalUrl(text: string): string | undefined {
  const match = /https?:\/\/(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1?\]):\d{2,5}(?:\/[^\s'"<>)\]]*)?/i.exec(text)
  if (!match) return undefined
  return match[0].replace('0.0.0.0', 'localhost').replace(/[.,;:]+$/, '')
}
