export function openEditor(path: string) {
  const editor =
    process.env.VISUAL ||
    process.env.EDITOR ||
    (process.platform === "win32" ? "code" : "vi")
  try {
    Bun.spawn([editor, path], { stdin: "ignore", stdout: "ignore", stderr: "ignore" })
  } catch {}
}

export async function exportTranscript(text: string): Promise<string | undefined> {
  try {
    const stamp = new Date().toISOString().replace(/[:.]/g, "-")
    const path = `agent-transcript-${stamp}.md`
    await Bun.write(path, text)
    return path
  } catch {
    return undefined
  }
}
