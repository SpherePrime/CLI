const BUILTIN_PRIME_SKILLS = new Set([
  'jq',
  'prime-config',
  'prime-designer',
  'prime-docx',
  'prime-gamedev',
  'prime-hooks',
  'prime-pdf',
  'prime-pptx',
  'prime-xlsx'
])

export type SettingsSkillEntry = {
  id?: string
  name?: string
  Name?: string
  description?: string
  label?: string
  source?: string
  path?: string
  error?: unknown
}

export function isBuiltinSkill(item: SettingsSkillEntry | null | undefined): boolean {
  if (!item || typeof item !== 'object') return false
  if (item.source === 'system') return true
  if (typeof item.id === 'string' && item.id.startsWith('prime://')) return true
  if (typeof item.label === 'string' && item.label.startsWith('system:')) return true
  const name = String(item.name || item.Name || '').trim().toLowerCase()
  if (!name) return false
  if (item.source === 'user' || item.source === 'project') return false
  return BUILTIN_PRIME_SKILLS.has(name)
}

export function filterVisibleSettingsSkills(
  runtime: unknown,
  disabledSkills: string[]
): SettingsSkillEntry[] {
  const runtimeList: SettingsSkillEntry[] = Array.isArray(runtime) ? runtime : []
  const visibleRuntime = runtimeList.filter((item) => !isBuiltinSkill(item))
  const seenNames = new Set(
    visibleRuntime
      .map((item) => String(item.name || item.Name || item.id || '').trim().toLowerCase())
      .filter(Boolean)
  )
  const runtimeBuiltinNames = new Set(
    runtimeList
      .filter((item) => isBuiltinSkill(item))
      .map((item) => String(item.name || item.Name || item.id || '').trim().toLowerCase())
      .filter(Boolean)
  )
  const disabledEntries: SettingsSkillEntry[] = []
  for (const rawName of disabledSkills) {
    const normalized = String(rawName || '').trim().toLowerCase()
    if (!normalized) continue
    if (seenNames.has(normalized)) continue
    if (runtimeBuiltinNames.has(normalized) || BUILTIN_PRIME_SKILLS.has(normalized)) continue
    seenNames.add(normalized)
    disabledEntries.push({ name: rawName })
  }
  return [...visibleRuntime, ...disabledEntries]
}
