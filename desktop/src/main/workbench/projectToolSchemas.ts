import { z } from 'zod'

export const identifier = z.string().uuid()
export const noArguments = z.strictObject({})
export const idArguments = z.strictObject({ id: identifier })
const shortText = z.string().trim().min(1)
const clock = z.string().regex(/^([01]\d|2[0-3]):[0-5]\d$/)
export const automationFields = {
  title: shortText.max(160),
  prompt: shortText.max(100000),
  enabled: z.boolean().default(true),
  trigger: z.discriminatedUnion('type', [
    z.strictObject({ type: z.literal('interval'), minutes: z.number().min(1).max(525600) }),
    z.strictObject({ type: z.literal('daily'), time: clock }),
    z.strictObject({
      type: z.literal('weekly'),
      time: clock,
      days: z.array(z.number().int().min(0).max(6)).min(1).max(7)
    }),
    z.strictObject({
      type: z.enum(['filesChanged', 'runFailed']),
      cooldownMinutes: z.number().min(1).max(525600)
    })
  ])
}
export const automationSave = z.strictObject({ id: identifier.optional(), ...automationFields })
export const automationUpdate = z.strictObject({
  id: identifier,
  title: automationFields.title.optional(),
  prompt: automationFields.prompt.optional(),
  enabled: z.boolean().optional(),
  trigger: automationFields.trigger.optional()
})
export const launchSave = z.strictObject({
  id: identifier.optional(),
  name: shortText.max(80),
  command: shortText.max(4000).refine((value) => !/[\r\n\0]/.test(value)),
  url: z.string().trim().max(2000).optional()
})
export const teamCreate = z.strictObject({
  title: shortText.max(120),
  goal: z.string().max(8000).default(''),
  concurrency: z.number().int().min(1).max(6).default(2),
  tasks: z
    .array(z.strictObject({ title: shortText.max(120), instruction: shortText.max(20000) }))
    .min(1)
    .max(20)
})
export const terminalCreate = z.strictObject({
  title: shortText.max(120).optional(),
  cols: z.number().int().min(20).max(500).optional(),
  rows: z.number().int().min(5).max(300).optional()
})
const source = z
  .strictObject({
    id: shortText.max(8192).optional(),
    kind: z.enum(['file', 'selection']),
    path: shortText.max(4096).refine((value) => !/[\0]/.test(value)),
    content: z.string().max(12000).optional(),
    startLine: z.number().int().positive().optional(),
    endLine: z.number().int().positive().optional()
  })
  .refine(
    (value) =>
      value.kind !== 'selection' ||
      (typeof value.content === 'string' &&
        value.startLine !== undefined &&
        value.endLine !== undefined &&
        value.endLine >= value.startLine),
    'Selection requires content and a valid line range'
  )
export const contextUpdate = z.strictObject({
  enabled: z.boolean().optional(),
  goal: z.string().max(4000).optional(),
  constraints: z.string().max(4000).optional(),
  criteria: z.string().max(4000).optional(),
  sources: z.array(source).max(8).optional()
})
