import type { WorkbenchTool } from '../workbench/tools'
import type { RemoteConnection } from './connection'
import { sshCommandTools } from './executionCommands'
import { sshFileTools } from './executionFiles'

export function createSshExecutionTools(
  resolve: (
    workspaceId: string,
    profileId?: string
  ) => RemoteConnection | undefined | Promise<RemoteConnection | undefined>
): WorkbenchTool[] {
  return [...sshCommandTools, ...sshFileTools].map((tool) => ({
    ...tool,
    description:
      tool.description +
      ' Available from any chat: provide profileId from ssh_profiles to select a saved server; otherwise use the current SSH project.',
    inputSchema: {
      ...tool.inputSchema,
      properties: {
        ...(tool.inputSchema.properties as Record<string, unknown>),
        profileId: {
          type: 'string',
          minLength: 1,
          description:
            'Saved SSH profile id from ssh_profiles. Required in chats without an SSH project.'
        }
      }
    },
    async run(args, context) {
      if (!context.workspaceId) throw new Error('SSH workspace binding is required')
      if (
        args.profileId !== undefined &&
        (typeof args.profileId !== 'string' || !args.profileId.trim())
      )
        throw new Error('Invalid SSH profileId')
      const connection = await resolve(context.workspaceId, args.profileId as string | undefined)
      if (!connection?.root) throw new Error('SSH workspace is disconnected')
      return tool.run(args, connection)
    }
  }))
}
