import type { TerminalManager } from './manager'

type ToolArguments = Record<string, unknown>
export type TerminalTool = {
  name: string
  description: string
  inputSchema: Record<string, unknown>
  run: (args: unknown, workspaceId?: string) => Promise<{ type: 'text'; text: string }[]>
}

function text(value: unknown) {
  return [{ type: 'text' as const, text: JSON.stringify(value) }]
}

function argumentsObject(value: unknown): ToolArguments {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw new Error('Terminal arguments must be an object')
  return value as ToolArguments
}

function stringArgument(args: ToolArguments, key: string) {
  const value = args[key]
  if (typeof value !== 'string' || !value.trim())
    throw new Error(`Terminal ${key} must be a non-empty string`)
  return value
}

function workspace(args: ToolArguments, boundWorkspace?: string) {
  const workspaceId = stringArgument(args, 'workspaceId')
  if (boundWorkspace !== undefined && workspaceId !== boundWorkspace)
    throw new Error('Terminal workspace access denied')
  return workspaceId
}

const baseProperties = {
  workspaceId: { type: 'string', minLength: 1, description: 'Project workspace ID' },
  id: { type: 'string', minLength: 1, description: 'Terminal ID from terminal_list' }
}

function schema(extra: Record<string, unknown> = {}, required: string[] = ['workspaceId', 'id']) {
  return {
    type: 'object',
    properties: { ...baseProperties, ...extra },
    required,
    additionalProperties: false
  }
}

export function terminalTools(manager: TerminalManager): TerminalTool[] {
  return [
    {
      name: 'terminal_list',
      description:
        'List project terminal sessions, ownership and exit status. Use terminal_create to open a new agent-owned terminal. Existing user-owned terminals require explicit user handover.',
      inputSchema: schema({}, ['workspaceId']),
      run: async (value, boundWorkspace) => {
        const args = argumentsObject(value)
        return text(manager.list(workspace(args, boundWorkspace)))
      }
    },
    {
      name: 'terminal_read',
      description:
        'Read the bounded terminal output journal and its sequence watermark, owner and exit status.',
      inputSchema: schema(),
      run: async (value, boundWorkspace) => {
        const args = argumentsObject(value)
        return text(manager.read(stringArgument(args, 'id'), workspace(args, boundWorkspace)))
      }
    },
    {
      name: 'terminal_write',
      description:
        'Write raw input to a terminal explicitly handed to the agent by the user. Include carriage return to run a command in the terminal shell. Fails while the user owns the terminal.',
      inputSchema: schema({ data: { type: 'string', maxLength: 1048576 } }, [
        'workspaceId',
        'id',
        'data'
      ]),
      run: async (value, boundWorkspace) => {
        const args = argumentsObject(value)
        if (typeof args.data !== 'string') throw new Error('Terminal data must be a string')
        return text(
          manager.write(
            stringArgument(args, 'id'),
            args.data,
            'agent',
            workspace(args, boundWorkspace)
          )
        )
      }
    },
    {
      name: 'terminal_resize',
      description: 'Resize a running terminal. Columns are bounded to 20–500 and rows to 5–300.',
      inputSchema: schema(
        {
          cols: { type: 'integer', minimum: 20, maximum: 500 },
          rows: { type: 'integer', minimum: 5, maximum: 300 }
        },
        ['workspaceId', 'id', 'cols', 'rows']
      ),
      run: async (value, boundWorkspace) => {
        const args = argumentsObject(value)
        if (!Number.isInteger(args.cols) || !Number.isInteger(args.rows))
          throw new Error('Invalid terminal dimensions')
        return text(
          manager.resize(
            stringArgument(args, 'id'),
            args.cols as number,
            args.rows as number,
            workspace(args, boundWorkspace)
          )
        )
      }
    }
  ]
}
