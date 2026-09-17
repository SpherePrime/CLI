export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (process.platform === "win32") {
      const proc = Bun.spawn(["clip"], { stdin: "pipe" })
      proc.stdin.write(text)
      await proc.stdin.end()
      await proc.exited
      return proc.exitCode === 0
    }
    if (process.platform === "darwin") {
      const proc = Bun.spawn(["pbcopy"], { stdin: "pipe" })
      proc.stdin.write(text)
      await proc.stdin.end()
      await proc.exited
      return proc.exitCode === 0
    }
    const proc = Bun.spawn(["xclip", "-selection", "clipboard"], { stdin: "pipe" })
    proc.stdin.write(text)
    await proc.stdin.end()
    await proc.exited
    return proc.exitCode === 0
  } catch {
    return false
  }
}
