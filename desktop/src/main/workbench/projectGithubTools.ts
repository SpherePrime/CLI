import type { WorkbenchTool } from './tools'
import { z } from 'zod'
import type { GithubRequest } from '../../shared/github'

export function createGitHistoryTools(
  execute: (workspaceId: string, input: GithubRequest) => Promise<unknown>
): WorkbenchTool[] {
  const sha = z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i)
  return [
    {
      action: 'history' as const,
      name: 'git_history',
      schema: z.strictObject({}),
      description:
        'Read up to 100 existing project commits from local and remote-tracking branches, including the current HEAD. Works without GitHub authentication.'
    },
    {
      action: 'showCommit' as const,
      name: 'git_show_commit',
      schema: z.strictObject({ sha }),
      description:
        'Inspect an existing project commit and its diff. Supply its full SHA from git_history. Returns currentHead for a later restore.'
    },
    {
      action: 'restoreCommit' as const,
      name: 'git_restore_commit',
      schema: z.strictObject({ sha, expectedHead: sha }),
      description:
        'Restore tracked project files to a selected historical commit by creating a new local commit. Preserves existing history and never pushes. Requires a clean repository and expectedHead from the latest preview. This modifies project files and repository history; use only when the user requests restoring this state.'
    }
  ].map<WorkbenchTool>(({ action, name, schema, description }) => ({
    name,
    description,
    inputSchema: z.toJSONSchema(schema),
    run: async (args, context) => {
      if (!context.workspaceId) throw new Error('Workspace binding is required')
      const parsed = schema.parse(args)
      return [
        {
          type: 'text',
          text: JSON.stringify(
            await execute(context.workspaceId, { action, ...parsed } as GithubRequest)
          )
        }
      ]
    }
  }))
}

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
