import { posix } from 'node:path'
import type { ToolContent } from '../browser/tools'
import type { WorkbenchTool } from '../workbench/tools'
import type { RemoteConnection } from './connection'
import { remotePath } from './security'

export type ExecutionDefinition = Omit<WorkbenchTool, 'run'> & {
  run(args: Record<string, unknown>, connection: RemoteConnection): Promise<ToolContent[]>
}

export function text(value: unknown): ToolContent[] {
  return [{ type: 'text', text: typeof value === 'string' ? value : JSON.stringify(value) }]
}

export function string(args: Record<string, unknown>, key: string, fallback?: string): string {
  const value = args[key] ?? fallback
  if (typeof value !== 'string' || value.includes('\0')) throw new Error(`Invalid ${key}`)
  return value
}

export function integer(
  args: Record<string, unknown>,
  key: string,
  fallback: number,
  maximum: number,
  minimum = 1
) {
  const value = args[key] ?? fallback
  if (typeof value !== 'number' || !Number.isInteger(value) || value < minimum || value > maximum)
    throw new Error(`Invalid ${key}: expected ${minimum}–${maximum}`)
  return value
}

export function relativePath(connection: RemoteConnection, value: string) {
  const path = posix.isAbsolute(value) ? posix.relative(connection.root, value) : value
  remotePath(connection.root, path)
  return path
}

export function schema(properties: Record<string, unknown>, required: string[] = []) {
  return { type: 'object', properties, required, additionalProperties: false }
}

export const pathProperty = {
  type: 'string',
  description: 'Remote project path, relative to its root or absolute inside it.'
}
export const stringProperty = { type: 'string' }
export const maxOutput = 256 * 1024
