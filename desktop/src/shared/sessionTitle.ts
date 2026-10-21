export function sessionTitle(title: string | undefined, emptyTitle: string): string {
  const value = title?.trim()
  return !value || /^(untitled session|new session|new chat|новый чат)$/i.test(value)
    ? emptyTitle
    : value
}
