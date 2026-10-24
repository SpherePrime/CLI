# Agent Orchestration Implementation Plan

> For agentic workers: use subagent-driven-development to implement the isolated tasks below and requesting-code-review for final verification.

**Goal:** Parallel bounded agent teams with peer messaging, persisted conversations, a right-side agent chat, and configurable defaults.

**Architecture:** Extend the existing coordinator and background registry. Expose workspace operations through the existing embedded engine/native bridge. Add the inspector tab and settings using existing components.

**Tech Stack:** Go, SQLite-backed sessions/messages, Rust Tauri host, React/TypeScript, Vite.

**Spec:** `docs/superpowers/specs/2026-10-24-agent-orchestration.md`

## Global Constraints

- Work on main; preserve the four existing installer/update changes.
- No new code comments, dependencies, attribution trailers, branches or worktrees.
- Default enabled, ten active additional workers per primary chat.
- Permissions and membership checks apply to every worker operation.
- Keep the ordinary dev window available after native tests.

## Task 1: Configuration and workspace transport

- [ ] Add persisted config fields and a bounded editable executor role.
- [ ] Add focused config/transport tests before behavior changes.
- [ ] Route roster/messages/send/stop through the optional runtime interface.
- [ ] Validate required fields, team membership and missing runtimes.
- [ ] Document the new operations and defaults.

## Task 2: Runtime orchestration

- [ ] Add exported worker DTO/interface in internal/agent.
- [ ] Test per-root concurrency, queue/cancel and disabled behavior.
- [ ] Register synchronous and asynchronous agent launches consistently.
- [ ] Add worker names and assignments, safe executor selection and prompts.
- [ ] Implement team-scoped communication and follow-up reuse.
- [ ] Deliver completion to an idle parent without duplicating consumed results.
- [ ] Recover historical child chats with truthful inactive states.

Interfaces:

```go
type AgentOrchestrator interface {
    AgentWorkers(context.Context, string) ([]AgentWorker, error)
    SendAgentWorkerMessage(context.Context, string, string, string) (AgentWorker, error)
    StopAgentWorker(context.Context, string, string) error
}
```

## Task 3: Desktop experience

- [ ] Add typed worker contracts and polling/events scoped to the primary chat.
- [ ] Add Agents to the right inspector, retaining files/changes/browser behavior.
- [ ] Render worker history, status, follow-up, stop and recovery states.
- [ ] Add enable and concurrency settings with defaults/persistence.
- [ ] Add concise RU/EN copy and responsive/reduced-motion styles.

## Integration and completion

- [ ] Review every task against the specification and fix findings.
- [ ] Run focused Go and renderer checks, then the production renderer build.
- [ ] Run isolated provider/native acceptance with multiple simultaneous workers.
- [ ] Commit and push only the feature files to main.
- [ ] Rebuild the embedded runtime and open the normal desktop dev window.
