# Context on Demand for CLI and Desktop

Status: implemented in the shared Go runtime used by both the CLI and the Desktop engine.

Update 2026-11-05: documentation is optional for every tool. Valid calls execute directly without a documentation gate. Basic file, shell, question and background-job tools are available immediately in both modes and retain parameter documentation. Other tools keep compact cards and Smart Tools discovery. The gate and receipt-coverage requirements below describe the original design and are superseded by this update. Canonical validation, permissions and result projection remain in effect.

## Decision

Standard mode initially exposes compact cards for **every permitted tool**, not discovery-only routing. A tool's complete documentation must be delivered to the model before its first execution. The shared runtime enforces this dependency. Smart Tools retains discovery and applies the same documentation gate to discovered tools. Successful outputs carry compact, meaningful data/status; failures retain diagnostics and unusually large failures have recoverable full artifacts.

A card that hides nothing needs no separate read: when the compact card already carries the complete description and the complete canonical schema, the gate treats the tool as documented. Reading documentation for a tool whose card is already complete would cost a round trip without adding information. Runtime routing and discovery tools (`read_tool_documentation`, `read_tool_result`, `search_skills`, `search_tools`, `search_mcp`, `search_instructions`, `read_instruction`) ship their complete contract inline for the same reason.

System prompt text, templates, mandatory instructions, permission semantics and stored conversation results remain unchanged. No new mode/default migration is required.

## Existing integration boundaries

- `internal/agent/coordinator.go`: `buildTools` filters local/MCP registries and applies `hookedTool`; `remote_workspace.go` builds a distinct Desktop/SSH registry and returns early.
- `internal/agent/agent.go`: `Run.PrepareStep` refreshes tools and calls `smartToolSelection`; `OnToolResult` persists full results. `preparePrompt`, `getSessionMessages`, `Summarize` determine retained context. A coordinator/session-agent object can serve multiple session IDs.
- `vendordeps/fantasy/agent.go`: `prepareTools` produces provider definitions; `validateAndRepairToolCall` and `executeSingleTool` use executable tools. Generate and Stream have separate orchestration paths; Stream dispatches parallel calls after completing all call callbacks.
- Today `ToolInfo` only exposes properties/required. MCP `Tool.Info` and Anthropic/Google adapters lose root schema keywords. Fantasy validation checks JSON and required fields only. These are prerequisites, not safe foundations for lossy schemas.
- `tools/mcp/tools.go` currently drops MCP `isError`; `mcp-tools.go` returns ordinary text responses. Shell nonzero exit can also return an ordinary text response. Neither nil Go error nor HTTP success proves operational success.
- `context_breakdown.go` counts all registered definitions and stored result text, while `Run` already estimates selected tools. Both need one prepared-request representation.

## Tool definitions and documentation

Keep immutable canonical definitions (full prose, full root input schema, provider options, identity and revision) separate from model presentation and executable handlers. Add full-root schema support with properties/required fallback for existing built-ins. Never mutate shared `Info` maps or replace actual arguments with an unvalidated generic JSON string.

Compact cards retain names, complete structural argument schemas and constraints. Strip only documentation annotations (`description`, `title`, `examples`, `$comment`) on schema objects, never object property names having those spellings, enum data, defaults or reference targets. Use a curated built-in short summary; use a deterministic first sentence capped at 240 Unicode characters for external descriptions. Append the documentation-read requirement. Unknown schema locations are retained unchanged. Full docs contain the original description, full canonical schema, version and identity. There is no arbitrary schema eviction/working-set limit in this scope.

Introduce runtime control tools:

- `read_tool_documentation(tool_name, offset=0, limit=32768)`: exact permitted canonical name, byte offsets at UTF-8 boundaries, bounded complete-document pages, next offset, total bytes, revision. Metadata records trusted page identity/range/hash. Read-only; cannot discover hidden Smart Tools or grant permissions.
- `read_tool_result(artifact_id, offset=0, limit=16384)`: reads an opaque current-session artifact, returns exact text page and continuation. No arbitrary filesystem path.

These two tiny controls carry their complete documentation in their inline definitions, so they bootstrap without recursion. They are the only inline-documentation exception; all existing tools, including search, instructions, skills and retrieval-like domain tools, use compact cards and the gate. Controls are present in both modes and both registry builders, independent of model-editable allowlists, but can access only the final permitted/selected registry and current session artifacts. Disabling a domain tool cannot be bypassed through a control.

## Runtime gate

At every outbound model request, construct an immutable step snapshot containing workspace/session scope, current permitted/selected identities, revisions, and the documentation coverage present in **that exact final request history**. Only trusted runtime-generated documentation tool results with matching bytes/hash/revision count. A user message, search hit, summary claiming docs were read, or arbitrary MCP metadata never counts.

An ordinary tool call proceeds only if the snapshot proves all pages of its current complete documentation were in the request which produced that call. Otherwise return a nonfatal error `tool_documentation_required` with the exact read call; no hooks, permission prompt or handler runs. Do not execute automatically after loading documentation: the model must issue the operation again on a later request. A docs read and an operation in one response never satisfy this dependency, irrespective of sequential/parallel scheduling. Already-authorized parallel operations remain parallel.

Order: active-name check -> documentation snapshot gate -> canonical schema validation/repair -> hooks -> revalidate hook-rewritten arguments -> existing permission checks -> handler. Repair cannot waive the gate or activate another tool. Recheck the latest registered identity/revision immediately before entering the handler chain; if changed, fail closed and require fresh docs. Provider-side executable services cannot be intercepted locally: do not expose provider-executed domain tools in this policy until an adapter can enforce the gate before provider execution. Surface an explicit unsupported-policy diagnostic, never claim they are protected.

## Lifetime and isolation

- Scope is backend/workspace identity plus real session ID; children, agents, projects and SSH profiles do not share read receipts or artifacts.
- Rebuild coverage from retained structured results each step/run; no process-global or turn-global `hasRead` flag. Normal new user turns retain valid docs if the complete bytes remain in context; Smart discovery still resets per turn.
- Documentation/schema/version changes invalidate matching coverage. Reconnection alone need not invalidate unchanged docs; a server identity/config/implementation revision change does.
- Summarization, history truncation or removal of any page removes the proof automatically. Do not exempt docs from compaction indefinitely or treat summary text as complete docs.
- Restart reconstructs coverage from persisted typed metadata and matching content that survives history preparation; older records without trusted provenance require rereading. Add optional versioned result metadata, preserving old message decoding.
- Documentation pages and explicit result-retrieval pages are not shortened by output projection. They must fit as pages; if complete docs cannot fit retained context, fail with an actionable context-capacity diagnostic, not an infinite read loop.

## Output projection

Preserve full results in existing storage/events. Project a deep copy of outgoing tool messages in `PrepareStep`, covering previous stored history and current fantasy step history. Preserve call pairing, names, error variants, StopTurn, metadata and media. Do not compact in `OnToolResult`, which is the persistence path.

Introduce typed outcomes: `success`, `failure`, `running`, `unknown`. Failure wins over every other status. Built-ins provide semantic outcomes; shell uses actual exit/interruption, MCP carries protocol `isError`, Desktop supplies known domain status where its payload has operational errors. Pending jobs remain running, not successful. Unknown third-party output is not guessed successful from HTTP/nil error or text keywords and remains lossless.

Success adapters preserve required data: reads/searches return requested content, line ranges, paths, refs and cursors; mutations return changed targets/counts and diagnostics; commands return exit code, cwd, relevant stdout/stderr and artifact reference; jobs return job ID/status/continuation; screenshots retain media and selectors. No generic `OK` replacement and no additional model summarization call. Default success inline text budget is 8 KiB; exceeding it requires a durable artifact and deterministic preview, with explicit page retrieval. Small meaningful outputs stay unchanged.

Failures and unknown outcomes are not success-summarized. Errors up to 64 KiB remain complete; larger errors retain an explicit error label, exception/exit/status, first 16 KiB and last 16 KiB, original byte count and recoverable artifact reference. Do not call that excerpt complete. Output storage failure means retain the full original output and report the storage problem; never silently discard diagnostics. Existing shell truncation must preserve the raw stream in an artifact before loss. Hook-added context, protocol errors and permission diagnostics are never dropped by success adapters.

## Artifacts and SSH

Store full text atomically under the engine-owned data directory, keyed by workspace/session and opaque artifact ID; manifest records call ID, outcome, checksum and byte count. Reject traversal, symlinks, foreign sessions and stale/missing IDs. No shared temp paths or arbitrary local `view` instructions. Artifacts live with the session and are deleted on session deletion; abandoned temporary files are cleaned on backend startup. Existing shell spill handles must be imported or safely referenced by this store before projection.

The Go runtime owning a conversation owns its output artifacts. Desktop remote-workspace tools operate through Desktop MCP/SSH, but result retrieval stays in the owning Go engine; it does not expose local file tools in SSH projects or fetch another SSH root. If the entire engine runs remotely, the same controls/storage run there. This avoids dual copies and transport-dependent path assumptions.

## Compatibility and acceptance

Ship through normal Go runtime and Desktop engine packaging / `prime update`; no installation-local patch. Preserve Smart Tools false/true and existing permission configuration. System prompt and template bytes must remain identical for equivalent configuration. This does not lower provider tool-count limits: Standard still sends all cards plus two controls; Smart discovery remains the answer to a provider cap, not silent conversion of Standard.

Capture real serialized requests for Chat Completions, Responses, Anthropic and Google fixtures. Verify canonical constraints and unchanged prompts, initial compact catalogs, discovery, mandatory gate, no same-response race, restart/compaction invalidation, hooks/permissions, full persisted versus compact outgoing results, operational failures over successful transport, UTF-8 spill retrieval/storage failure, session isolation and Desktop local/SSH parity. Context reporting must count final prepared definitions/messages and label token estimates separately from provider usage.

## Deferred

Automatic semantic history replacement, system/skill prompt shortening, provider-cache tuning, schema working-set eviction and provider-specific hard-cap routing. None is required for the approved gate and output policy.
