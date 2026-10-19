import type { WorkbenchTool } from './tools'
import { z } from 'zod'

export function createGithubReadTools(
  execute: (workspaceId: string, input: { action: string; number?: number }) => Promise<unknown>
): WorkbenchTool[] {
  return (
    ['status', 'issues', 'pulls', 'branches', 'issue', 'pull', 'diff', 'failedLogs'] as const
  ).map((action) => {
    const detail = ['issue', 'pull', 'diff', 'failedLogs'].includes(action)
    const schema = detail
      ? z.strictObject({ number: z.number().int().positive() })
      : z.strictObject({})
    return {
      name: `github_${action === 'failedLogs' ? 'failed_logs' : action}`,
      description: `Read GitHub ${action} for this project using its existing GitHub CLI account. ${detail ? 'Provide the issue or pull request number.' : 'Does not modify the repository.'}`,
      inputSchema: z.toJSONSchema(schema),
      run: async (args, context) => {
        if (!context.workspaceId) throw new Error('Workspace binding is required')
        const parsed = schema.parse(args)
        return [
          {
            type: 'text',
            text: JSON.stringify(await execute(context.workspaceId, { action, ...parsed }))
          }
        ]
      }
    }
  })
}
