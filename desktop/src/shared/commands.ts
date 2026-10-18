export type Command = {
  name: string
  content: string
  arguments: { id: string; title: string; required: boolean; description?: string }[]
  clientId?: string
  promptId?: string
  skill?: any
}
export function normalizeCommand(value: any): Command {
  return {
    name: value.Name || value.name || value.title || value.id,
    content: value.Content || value.content || value.prompt || '',
    arguments: (value.Arguments || value.arguments || []).map((argument: any) => ({
      id: argument.ID || argument.id,
      title: argument.Title || argument.title || argument.ID || argument.id,
      required: Boolean(argument.Required || argument.required),
      description: argument.Description || argument.description
    })),
    clientId: value.client_id,
    promptId: value.prompt_id,
    skill: value.Skill || value.skill
  }
}
export function renderCommand(content: string, args: Record<string, string>): string {
  return content.replace(/\$([A-Z][A-Z0-9_]*)/g, (match, key) => args[key] ?? match)
}
