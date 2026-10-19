import { z } from 'zod'
import type { AutomationManager } from '../automations/manager'
import type { AutomationRequest } from '../../shared/automations'

const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('list') }),
  z.object({ action: z.literal('save'), input: z.unknown() }),
  z.object({ action: z.literal('remove'), id: z.string().uuid() }),
  z.object({ action: z.literal('run'), id: z.string().uuid() }),
  z.object({ action: z.literal('cancel'), runId: z.string().uuid() })
])

export function automationOperation(manager: AutomationManager, root: string, input: unknown) {
  if ((JSON.stringify(input)?.length || 0) > 200000)
    throw new Error('Запрос автоматизации слишком большой')
  const request = requestSchema.parse(input)
  return manager.handle({ ...request, root } as AutomationRequest)
}
