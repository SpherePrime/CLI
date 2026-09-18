import type { FileChange } from "../client"
import type { ToolState } from "../context/timeline"

export function filePathOf(args: Record<string, unknown>): string | undefined {
  const path = args.path ?? args.file ?? args.directory ?? args.dest
  if (typeof path === "string") return path
  return undefined
}

export function fileLabel(args: Record<string, unknown>): string | undefined {
  const path = filePathOf(args) ?? args.working_dir
  if (typeof path === "string") {
    const parts = (path as string).split("/").pop()?.split("\\").pop() ?? path
    return parts || undefined
  }
  return undefined
}

export function describeToolAction(name: string, args: Record<string, unknown> | undefined): string {
  const args_ = args ?? {}
  const path = filePathOf(args_)
  const file = fileLabel(args_)
  const command = typeof args_.command === "string" ? args_.command.trim() : undefined
  const run = name.toLowerCase()

  if (run.startsWith("plugin_")) return `plugin ${name.slice("plugin_".length).split("_")[0]} → ${name}`
  if (run.startsWith("mcp_")) return `mcp ${name.slice("mcp_".length).split("_")[0]} → ${name}`
  switch (run) {
    case "read_file":
      return path ? `read ${path}` : "read file"
    case "read_many_files":
    case "read_some_files":
      return "read files (batch)"
    case "write_file":
    case "create_file":
      return path ? `write ${path}` : "write file"
    case "edit_file":
    case "patch_file":
    case "apply_patch":
      return file ? `edit ${file}` : "edit files (patch)"
    case "delete_path":
    case "delete_file":
    case "remove":
      return path ? `delete ${path}` : "delete"
    case "create_dir":
    case "mkdir":
      return path ? `create dir ${path}` : "create dir"
    case "move":
      return path ? `move ${path}` : "move"
    case "list_dir":
    case "ls":
      return path ? `list ${path}` : "list dir"
    case "glob":
    case "find":
      return `find ${typeof args_.pattern === "string" ? args_.pattern : ""}`.trim()
    case "grep":
      return `grep ${typeof args_.query === "string" ? args_.query : ""}`.trim()
    case "shell":
    case "terminal":
    case "run":
      return command ? `run ${shorten(command, 60)}` : "run command"
    case "git":
      return command ? `git ${shorten(command, 60)}` : "git"
    case "test":
    case "run_tests":
    case "run_checks":
      return "run checks (tests/lint/format)"
    case "ask_user":
      return "ask user"
    case "update_plan":
      return "update plan"
    case "http_fetch":
    case "fetch":
      return `fetch ${typeof args_.url === "string" ? args_.url : ""}`.trim()
    case "view_image":
      return file ? `view image ${file}` : "view image"
    case "lsp":
      return `lsp ${typeof args_.action === "string" ? args_.action : ""}`.trim()
    case "process":
      return `process ${typeof args_.action === "string" ? args_.action : ""}`.trim()
    case "dependency":
      return `dependency ${typeof args_.action === "string" ? args_.action : ""}`.trim()
    default:
      return name
  }
}

export function argsPreview(args: unknown): string {
  const raw = JSON.stringify(args)
  if (!raw) return "{}"
  return raw.length > 240 ? `${raw.slice(0, 240)}…` : raw
}

export function fileChangesSummary(changes: FileChange[]): string {
  return changes
    .map((change) => {
      const ops = [change.additions, change.deletions].filter((n) => typeof n === "number" && n > 0).join("/")
      const stats = ops ? ` (+${ops})` : ""
      return `${change.change} ${change.path}${stats}`
    })
    .join("\n")
}

export function toolStatusLabel(state: ToolState): string {
  switch (state) {
    case "queued":
      return "queued"
    case "running":
      return "running"
    case "waiting_permission":
      return "waiting for approval"
    case "ok":
      return "ok"
    case "failed":
      return "failed"
    case "denied":
      return "denied"
    case "cancelled":
      return "cancelled"
    case "timed_out":
      return "timed out"
  }
}

export function deriveToolState(ok: boolean, error: string | null | undefined, summary?: string | null): ToolState {
  if (ok) return "ok"
  const text = [error ?? "", summary ?? ""].join(" ").toLowerCase()
  if (text.includes("timed out") || text.includes("timeout")) return "timed_out"
  if (text.includes("denied") || text.includes("not permitted") || text.includes("permission")) return "denied"
  if (text.includes("cancel")) return "cancelled"
  return "failed"
}

export type DiffLine = { sign: "+" | "-" | " "; line: string }

export function diffLines(diff: string | null | undefined): DiffLine[] {
  if (!diff) return []
  const lines: DiffLine[] = []
  for (const raw of diff.split("\n")) {
    const line = raw.replace(/\r$/, "")
    if (line.startsWith("+")) lines.push({ sign: "+", line })
    else if (line.startsWith("-")) lines.push({ sign: "-", line })
    else lines.push({ sign: " ", line })
  }
  return lines
}

export function shorten(text: string, max: number): string {
  if (text.length <= max) return text
  return `${text.slice(0, max - 3)}…`
}