import type { ToolContent } from '../browser/tools'
import type { TerminalManager } from '../terminal/manager'
import { terminalTools } from '../terminal/tools'

export type WorkbenchToolContext = { workspaceId?: string }
export type WorkbenchTool = {
  name: string
  description: string
  inputSchema: Record<string, unknown>
  run: (args: Record<string, unknown>, context: WorkbenchToolContext) => Promise<ToolContent[]>
}

export function workbenchTerminalTools(manager: TerminalManager): WorkbenchTool[] {
  return terminalTools(manager).map((tool) => {
    const schema = tool.inputSchema as { properties: Record<string, unknown>; required: string[] }
    const { workspaceId: _, ...properties } = schema.properties
    return {
      ...tool,
      inputSchema: {
        ...schema,
        properties,
        required: schema.required.filter((key) => key !== 'workspaceId')
      },
      run: async (args, context) => {
        if (!context.workspaceId) throw new Error('Workspace binding is required')
        return tool.run({ ...args, workspaceId: context.workspaceId }, context.workspaceId)
      }
    }
  })
}
