const cmdStart = /^start(?:\.exe)?\s+(?:""\s+)?(?:\/(?:b|min|max|wait)\s+)*("[^"]+"|\S+)(.*)$/i

export function powershellLaunchCommand(command: string) {
  const match = cmdStart.exec(command.trim())
  if (!match) return command
  const [, file, rest] = match
  const target = file.startsWith('"') ? file : `"${file}"`
  const args = rest.trim()
  if (!args) return `Start-Process ${target}`
  return `Start-Process ${target} -ArgumentList '${args.replace(/'/g, "''")}'`
}
