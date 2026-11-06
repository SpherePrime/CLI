export function projectNameError(name: string): string | null {
  if (!name || name !== name.trim() || [...name].length > 80 || /[<>:"/\\|?*\u0000-\u001f\u007f]/.test(name) || name.endsWith('.') || /^(con|prn|aux|nul|com[1-9¹²³]|lpt[1-9¹²³])(?:\.|$)/i.test(name))
    return 'Введите корректное имя: до 80 символов, без служебных имён и символов пути'
  return null
}

export function projectCreationPath(base: string, name: string): string {
  return `${base}${base.includes('\\') ? '\\' : '/'}${name}`
}
