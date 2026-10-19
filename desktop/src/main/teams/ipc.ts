import { z } from 'zod'
import type { TeamManager } from './manager'
import type { TeamRequest } from '../../shared/teams'

const id = z.string().uuid()
const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('list') }),
  z.object({
    action: z.literal('create'),
    input: z.object({
      title: z.string().max(120),
      goal: z.string().max(8000),
      concurrency: z.number().int().min(1).max(6),
      tasks: z
        .array(z.object({ title: z.string().max(120), instruction: z.string().max(20000) }))
        .min(1)
        .max(20)
    })
  }),
  z.object({ action: z.literal('remove'), teamId: id }),
  z.object({ action: z.literal('stopTeam'), teamId: id }),
  z.object({ action: z.enum(['stopTask', 'retryTask']), teamId: id, taskId: id })
])

export function teamOperation(manager: TeamManager, root: string, input: unknown) {
  return manager.handle(root, requestSchema.parse(input) as TeamRequest)
}