# Context on Demand Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send compact tool cards and meaningful outputs in CLI/Desktop while enforcing complete documentation delivery before execution without changing system prompts.

**Architecture:** Keep canonical permitted tools separate from provider-facing definitions. Build a per-request immutable documentation gate from retained trusted results; project only outbound message copies after full result persistence. Use engine-owned, session-scoped artifacts for recoverable output.

**Tech Stack:** Existing Go agent/backend, vendored fantasy/provider adapters, vendored google/jsonschema-go validator, Desktop Rust MCP host and TypeScript presentation. No new dependency is required.

**Spec:** `docs/superpowers/specs/2026-11-04-context-on-demand-design.md`

## Global Constraints

- System prompt text, templates, mandatory instructions, permission semantics and stored conversation results remain unchanged.
- Standard mode initially exposes compact cards for **every permitted tool**, not discovery-only routing.
- Smart Tools retains discovery and applies the same documentation gate to discovered tools.
- No new mode/default migration is required.
- Failures and unknown outcomes are not success-summarized.
- Work only on `main`; no branches/worktrees, code comments, commits or pushes under the current authorization.
- This document is a plan, not evidence of implemented functionality or passing tests. All commands below are future verification commands.

## Review Focus

1. Same-response documentation read plus destructive call must never authorize the destructive call, even when scheduler order changes (Task 4).
2. Property keys named `description`, local `$ref`/`$defs`, root `additionalProperties`, and hook input rewrites must not lose validation (Tasks 1–2, 4).
3. Restarted sessions, summary-only history and cross-session MCP metadata must not manufacture read receipts (Tasks 3–4).
4. Desktop/SSH domain failure delivered via HTTP 200, and background shell nonzero exit, must remain failures with retrievable diagnostics (Tasks 5–7).
5. Missing/full/read-only artifact storage and UTF-8 page boundaries must preserve the raw response rather than emit a broken retrieval promise (Tasks 6–7).

## Repository baseline and concrete seams

At planning start `git fetch` succeeded, `main` matched `origin/main` (0/0), and the existing untracked design file was the only visible change. Recheck before any future implementation; preserve concurrent changes.

| Existing seam | Present behavior | Change boundary |
| --- | --- | --- |
| `internal/agent/coordinator.go:buildTools` | allowed built-ins/MCP, then hook wrappers | canonical catalog/control registration, no template edits |
| `internal/agent/remote_workspace.go:buildRemoteWorkspaceTools` | early distinct registry; no local view/bash | register controls here too; never depend on local view |
| `internal/agent/agent.go:Run.PrepareStep` | latest `a.tools`, search recovery, active selection, cache/media transforms, context estimate | final projection, immutable snapshot, request receipt |
| `internal/agent/smart_tool_selection.go` | searches reveal names from this run's `Steps`; `view`/instructions bootstrap | retain discovery semantics, add controls, gate all domain tools |
| `vendordeps/fantasy/agent.go:prepareTools` | root reconstructed from properties/required | separate `FunctionTool` presentation from executable registry |
| `validateToolCall` | only JSON + required keys | canonical schema validation; no relaxed compact schema |
| `Generate`, `Stream/processStepStream` | separate validation/dispatch paths; Stream parallelism | common pre-execution gate used by both |
| `executeSingleTool` | normalizes Go error/IsError/media and calls result callback | preserve raw typed outcome/provenance |
| `internal/agent/hooked_tool.go:Run` | hooks can rewrite input after fantasy validation | revalidate rewritten canonical input before permissions/handler |
| `internal/agent/tools/mcp/tools.go:RunToolWithProgress` | drops protocol IsError/structured content | preserve semantic outcome before output projection |
| `internal/agent/tools/bash.go`, `job_output.go`, `mcp_jobs.go` | results may be successful transport with failed operation | typed exit/job/protocol outcome |
| `internal/agent/context_breakdown.go` | scans registry + stored messages | consume same prepared request view as runtime |
| `internal/backend/context.go` | maps shared reporter to Desktop protocol | preserve mapping, add parity tests |

Inspect `vendordeps/fantasy/providers/openai/language_model.go` and `responses_language_model.go` (pass full schema), `providers/anthropic/anthropic.go:toTools` and `providers/google/google.go:toGoogleTools` (currently root reconstruction), and `providers/openaicompat` (delegates to OpenAI). Azure/Bedrock/OpenRouter/Vercel forwarding must remain compatible.

## Shared contracts and ownership

The following are planned interfaces, not existing APIs. Keep each concern in the file named by its task. New structs use exported fields below; JSON names use snake_case with `omitempty` for optional data. Do not repurpose `ClientMetadata` as authority.

```go
// vendordeps/fantasy/tool.go (the implementation must omit this explanatory comment)
type ToolOutcome string
const (
    ToolOutcomeUnknown ToolOutcome = "unknown"
    ToolOutcomeSuccess ToolOutcome = "success"
    ToolOutcomeFailure ToolOutcome = "failure"
    ToolOutcomeRunning ToolOutcome = "running"
)
```

Add optional `InputSchema map[string]any` to `ToolInfo`; existing properties/required remain a compatibility fallback. Add `Outcome ToolOutcome` to `ToolResponse`, `ToolResultContent` and `message.ToolResult`; the stored message field uses the fantasy ToolOutcome type to avoid duplicate enum definitions. Empty legacy values normalize to unknown. Add optional typed `Documentation *ToolDocumentationReceipt` on `ToolResponse`, `ToolResultContent`, and `message.ToolResult`, never reconstructed from external metadata:

```go
type ToolDocumentationReceipt struct {
    Version int
    Scope string
    ToolName string
    Identity string
    Revision string
    Offset int
    End int
    Total int
    PageSHA256 string
}
```

Only runtime control implementation sets Documentation. Sanitize this field from external adapters. Persist it in optional structured message fields; old messages decode with nil and remain usable but cannot authorize calls. `Scope` is derived from the backend/workspace data-directory identity and real session ID, never model arguments. `Identity` includes origin (builtin or MCP server/config identity), canonical name, and implementation version. `Revision` hashes original full description, canonical root schema and implementation version in deterministic JSON ordering. Prefix collisions of MCP tools are errors, not last-writer-wins.

```go
// vendordeps/fantasy/agent.go: add to PrepareStepResult
type PrepareStepResult struct {
    // existing fields retained; implementation adds no comments
    Model LanguageModel
    Messages []Message
    System *string
    ToolChoice *ToolChoice
    ActiveTools []string
    DisableAllTools bool
    Tools []AgentTool
    ToolDefinitions []Tool
    BeforeToolCall func(context.Context, ToolCall) error
}
```

Existing field types above match the current declaration; retain them verbatim. The new fields' signatures are the contract: non-nil `ToolDefinitions` replaces only outbound definitions; `BeforeToolCall` is invoked before repair, hooks or permission operations. The canonical active tools still supply validation and execution. nil preserves existing fantasy callers. Empty explicit definitions must not accidentally fall back to all tools. The gate callback must also run for repaired calls if their name changes.

```go
// internal/agent/tool_catalog.go
func canonicalToolSchema(info fantasy.ToolInfo) (map[string]any, error)
func compactToolDefinition(tool fantasy.AgentTool) (fantasy.FunctionTool, error)

// internal/agent/tool_documentation.go
type toolDocument struct {
    Name string
    Identity string
    Revision string
    Text string
}
func buildToolDocuments(scope string, tools []fantasy.AgentTool) (map[string]toolDocument, error)
func documentedTools(scope string, docs map[string]toolDocument, messages []fantasy.Message) map[string]string
func documentationGate(scope string, docs map[string]toolDocument, covered map[string]string, latest func() []fantasy.AgentTool) func(context.Context, fantasy.ToolCall) error
```

`documentedTools` returns tool-name -> revision only when complete [0,total) coverage of matching exact bytes is present as runtime-issued documentation results in the final outbound history. No result text search, substring heuristic or trust in MCP `ClientMetadata`. Each page verifies SHA-256 and matches canonical document text for its range. Pages from different versions never combine.

```go
// internal/agent/tool_artifacts.go
type toolArtifactStore interface {
    Put(context.Context, string, string, fantasy.ToolOutcome, []byte) (string, error)
    Read(context.Context, string, string, int, int) ([]byte, int, int, error)
    DeleteSession(context.Context, string) error
}
// Put arguments after ctx: scope, call ID, outcome, raw text.
// Read arguments after ctx: scope, artifact ID, byte offset, limit.

// internal/agent/tool_result_projection.go
func projectToolMessages(ctx context.Context, scope string, messages []fantasy.Message, artifacts toolArtifactStore) ([]fantasy.Message, error)
```

Treat projection's artifact-write error as a per-result fallback: retain original full text, surface diagnostic, and continue, not a failed whole turn. Fatal errors are invalid internal message pairings only. Controls return typed receipt for docs and page data for artifact reads; projection never shortens either kind of explicit page.

## Task 1: Preserve canonical root schemas and validate inputs

**Files:** modify `vendordeps/fantasy/tool.go`, `vendordeps/fantasy/content.go`, `vendordeps/fantasy/agent.go`, `internal/agent/tools/mcp-tools.go`; create `vendordeps/fantasy/tool_schema.go`, `vendordeps/fantasy/tool_schema_test.go`.

**Consumes:** existing `ToolInfo`/AgentTool. **Produces:** `InputSchema`, `CanonicalToolSchema(info ToolInfo) (map[string]any, error)` and `ValidateToolInput(info ToolInfo, input string) error`, exported from fantasy; internal `canonicalToolSchema` delegates to it.

- [ ] Write table tests in `tool_schema_test.go` for root additionalProperties=false, nested required, enum/type/limits, oneOf, internal refs, null/array root, bad JSON, missing required, unsupported dialect and external reference. Expected invalid input never calls handler. Regression fixture:

```go
func TestCanonicalSchemaRejectsExtraArguments(t *testing.T) {
    info := ToolInfo{Name: "change", InputSchema: map[string]any{
        "type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}},
        "required": []string{"path"}, "additionalProperties": false,
    }}
    if err := ValidateToolInput(info, `{"path":"a","unexpected":true}`); err == nil {
        t.Fatal("extra argument was accepted")
    }
}
```

- [ ] Run `go test ./vendordeps/fantasy -run 'TestCanonicalSchema|TestValidateToolInput' -count=1`; expect missing new API or permissive-validation failures.
- [ ] Preserve full MCP InputSchema by JSON deep copy; fallback constructs object with properties/required. Add `tool_schema.go` using vendored `google/jsonschema-go/jsonschema`: marshal canonical map -> unmarshal Schema -> `Resolve(nil)` -> `Validate(decodedJSON)`. Reject non-object inputs before handler. Do not apply defaults/coerce input; no remote loader. Cache resolved schemas by canonical hash with a bounded 512-entry synchronized cache. Compile failures produce tool-specific diagnostics and prevent execution, not validation bypass.

```go
var schemaValue jsonschema.Schema
if err := json.Unmarshal(schemaBytes, &schemaValue); err != nil { return err }
resolved, err := schemaValue.Resolve(nil)
if err != nil { return err }
var value any
if err := json.Unmarshal([]byte(input), &value); err != nil { return err }
if _, ok := value.(map[string]any); !ok { return errors.New("tool input must be an object") }
return resolved.Validate(value)
```

- [ ] Replace fantasy JSON/required-only validation with the new validator; revalidate repaired inputs. Run focused tests and `go test ./vendordeps/fantasy ./internal/agent/tools -count=1`.
- [ ] Review canonical maps for aliasing, local-ref support, large numbers and no network fetch. Preserve unsupported schema diagnostics rather than silently dropping keywords. No commit under current authorization.

## Task 2: Separate compact provider presentation from executable tools

**Files:** create `internal/agent/tool_catalog.go`, `internal/agent/tool_catalog_test.go`; modify `vendordeps/fantasy/agent.go`, `providers/anthropic/anthropic.go`, `providers/google/google.go`; create `vendordeps/fantasy/tool_presentation_test.go`, `providers/anthropic/tool_schema_test.go`, `providers/google/tool_schema_test.go`.

**Consumes:** Task 1 canonical schema. **Produces:** `compactToolDefinition`, ToolDefinitions override in both Generate/Stream, unchanged canonical execution registry.

- [ ] Add a fake AgentTool fixture whose long description and nested property description appear only in canonical data. Include properties literally named `description` and `$comment`, enum objects, default objects, `$defs` references and root constraints. Assert deep equality of original maps after preparing two requests concurrently.
- [ ] Run `go test ./internal/agent -run TestCompactTool -count=1` and `go test ./vendordeps/fantasy -run TestToolPresentation -count=1`; expect missing API.
- [ ] Implement schema-aware recursive annotation stripping (only schema nodes under properties/items/compositions/definitions and known schema keywords), deterministic external first-sentence summary capped at 240 runes, built-in short summaries, and appended `Read full documentation with read_tool_documentation before execution.` All remaining constraint JSON is identical. Never truncate serialized schema bytes.
- [ ] Wire `ToolDefinitions` through PrepareStep -> model call in both fantasy paths; validation/lookup continue using canonical active tools. Keep provider options/cache controls and tool choice names. Definition names must exactly equal active canonical names; reject duplicates/missing names. Bootstrap controls also appear in this explicit set.
- [ ] Preserve full root schema in Anthropic serialization, using existing raw-tool JSON path rather than lossy properties/required reconstruction. For Google assign `genai.FunctionDeclaration.ParametersJsonSchema = ft.InputSchema` and leave `Parameters` nil; this field exists in `vendordeps/genai/types.go:2477`. Validate provider acceptance with captured fixtures and emit an explicit unsupported-schema error for provider-rejected constraints. Never silently relax local validation. Capture serialized OpenAI Chat Completions and Responses plus Anthropic/Google definitions and compare fixture constraints. Run all relevant adapter tests.

## Task 3: Add versioned documentation pages and safe bootstrap controls

**Files:** create `internal/agent/tool_documentation.go`, `tool_documentation_tools.go`, `tool_documentation_test.go`; modify `vendordeps/fantasy/tool.go`, `content.go`, `internal/message/content.go` (ToolResult declaration), `internal/agent/agent.go:convertToToolResult`, `internal/message/message.go:ToAIMessage`, `internal/message/content.go` part JSON encoding and `internal/message/parts_json_test.go`.

**Consumes:** canonical docs and compact presentation. **Produces:** typed Documentation receipt end-to-end, `buildToolDocuments`, `documentedTools`, paged `read_tool_documentation`; result control registration shares Task 6 store.

- [ ] Write tests for complete/partial/overlapping pages, changed revision, content mismatch, fake user text, forged MCP metadata, canceled read, two session IDs and roundtrip through stored message encoding. Example expected state:

```text
Page [0,32768) only, total 40000 -> not documented
Page [32768,40000) added to same retained request -> documented
Second page replaced by summary text -> not documented
Both pages retained but revision changed -> not documented
```

- [ ] Run `go test ./internal/agent -run 'TestToolDocument|TestDocumentedTools' -count=1`; expect missing control/receipt behavior.
- [ ] Build deterministic document text from full original description, canonical schema, identity/revision. Add exact-name lookup only against the current selected/permitted snapshot. Page default 32768/max 32768 bytes; reject negative/out-of-range offsets and non-UTF-8 boundaries; page end backs off to UTF-8 boundary. Set receipt only after successful page construction; include total/next offset in text. External descriptions are untrusted tool data, not system instructions.
- [ ] Add optional typed fields to stored ToolResult and fantasy conversion so trusted provenance survives restart and same-run `toResponseMessages`. Existing metadata is retained independently. Check all constructors/error/media conversions and strip authority on external adapter ingress. No database column is needed if the existing JSON parts encoding preserves optional fields; prove this by roundtrip test rather than assume it.
- [ ] Define both controls' compact-but-complete inline contracts; mark only them as inline-documented in the catalog. Register after permitted domain filtering in local and remote builders, before final sorting; their closures see only current active tools and scope from runtime context. A control cannot grant execution or discover a hidden Smart tool. Re-run tests including old-message decoding.

## Task 4: Enforce a per-request gate across modes, hooks and parallel calls

**Files:** create `internal/agent/tool_documentation_gate.go`, `tool_documentation_gate_test.go`; modify `internal/agent/agent.go`, `smart_tool_selection.go`, `coordinator.go`, `remote_workspace.go`, `hooked_tool.go`; modify `vendordeps/fantasy/agent.go`; extend `vendordeps/fantasy/agent_parallel_tools_test.go`, `internal/agent/hooked_tool_test.go` and `smart_tool_selection_test.go`.

**Consumes:** Tasks 1–3 definitions/receipts. **Produces:** `documentationGate` and BeforeToolCall lifecycle; invariant enforced for every exposed local-executable tool.

- [ ] Add handler/permission/hook spies. First call without docs must return `tool_documentation_required` and all spies stay zero. Then read docs, issue a later model request and call again: existing hook/permission/handler path fires exactly once. Denial/halt/cancel semantics remain unchanged. Same-name revision change fails without dispatch.
- [ ] Add Stream and Generate tests with two calls in one response: documentation read then mutation, reverse order, sequential versus parallel=true, randomized control completion. All mutations stay blocked until a subsequent outbound request includes full docs. Add already-documented two-handler barrier test proving authorized concurrency remains.
- [ ] Run `go test ./internal/agent -run 'TestDocumentationGate|TestSmartTool|TestHookedTool' -count=1` and `go test ./vendordeps/fantasy -run 'Test.*Documentation|Test.*Parallel' -count=1`; expect missing gate enforcement.
- [ ] In final `PrepareStep`, after history/media transformations and output projection, build docs/coverage from final message copies, then assign an immutable callback. Selection: Standard all permitted cards plus controls; Smart controls plus existing search/instruction bootstrap cards and current discovered names. Search/domain bootstrap cards still require their docs; controls do not. Do not reuse mutable shared `a.tools` or global boolean for coverage. Never let a same-response control update the captured snapshot.

```go
covered := documentedTools(scope, documents, prepared.Messages)
prepared.BeforeToolCall = documentationGate(scope, documents, covered, a.tools.Copy)
```

- [ ] In fantasy Generate and Stream, check active name and gate before validation/repair. Represent rejection as invalid call with nonfatal typed error result so the model can recover; no automatic tool replay. Run gate again if repair changes the tool name. In hookedTool, use `ValidateToolInput(h.inner.Info(), result.UpdatedInput)` before hook approval/handler. Current-revision check belongs before hooks/permissions, using the latest permitted registry; hold a scoped read lease or verify generation against the exact handler snapshot so replacement cannot race dispatch.
- [ ] Test new user turn (docs retained, no reread), Smart turn discovery reset, auto-continue/retry, manual/automatic summarize, context-overflow recovery, two sessions on same sessionAgent, child agents and full backend recreation. Coverage is rebuilt from actual final history; failed summary does not discard still-retained proof. No retained complete document -> gate closes. Add explicit capacity error when a document's full pages cannot coexist with required context rather than endless paging.
- [ ] Inventory provider-defined and provider-executed tools. Disable exposure of un-gateable provider-executed domain services with explicit unsupported-policy diagnostics; do not count their post-execution callbacks as enforcement. Fake-provider tests verify no unprotected provider service is serialized. Verify system prompt bytes identical with/without compact presentation for equivalent config.

## Task 5: Preserve semantic outcomes before shortening outputs

**Files:** modify `vendordeps/fantasy/tool.go`, `content.go`, `agent.go`, `internal/message` ToolResult encoding, `internal/agent/tools/mcp/tools.go`, `mcp-tools.go`, `mcp_jobs.go`, `mcp_job.go`, `bash.go`, `job_output.go`, `internal/agent/background_wake.go:backgroundJobWakePrompt/mcpJobWakePrompt`; create `internal/agent/tools/tool_outcome_test.go`, `internal/agent/tools/mcp/tool_result_test.go`; modify `desktop/src-tauri/src/browser/mcp.rs` and the domain handlers it dispatches where they encode operational failures as ordinary text.

**Consumes:** ToolOutcome shared contract. **Produces:** end-to-end success/failure/running/unknown without transport-based guessing.

- [ ] Build fixtures for HTTP 200 + MCP isError=true (including empty/mixed-media content), nil Go error + shell exit 1, permission denial, hook halt, running MCP job, completed background error, interrupted shell, domain JSON status failed and unknown external text containing the word error in a successful data value. Expected failure always wins; lexical matching is prohibited.
- [ ] Run `go test ./internal/agent/tools/... -run 'Test.*Outcome|Test.*MCP.*Error' -count=1`; expect dropped MCP failure and shell misclassification.
- [ ] Preserve protocol IsError and structured content in MCP internal result; propagate to ToolResponse/ToolResultContent/message result for foreground, background, retrieval and wake paths. Make shell outcome derive from exit code/interruption and attach structured exit/cwd metadata before formatOutput truncation. Do not change StopTurn policy when changing outcome. Existing Go errors and IsError imply failure regardless declared outcome.
- [ ] Audit Desktop `browser/mcp.rs` dispatch and SSH/app/terminal methods: map actual domain errors to MCP isError or typed structured outcome at source. Keep partial payload and diagnostic details. Unknown third-party MCP result without explicit semantic contract stays unknown, even if transport/protocol succeeded; allow only explicit tool-specific success adapters to promote it.
- [ ] Add tests for typed fields surviving conversion, duplicate metadata merge with hooks, media/error combinations, hidden background wake continuations, and old-result compatibility. Run Go tools tests and `cargo test --lib` in `desktop/src-tauri` when implementation reaches this slice.

## Task 6: Add scoped recoverable artifacts and explicit page retrieval

**Files:** create `internal/agent/tool_artifacts.go`, `tool_artifacts_test.go`; complete `tool_documentation_tools.go:read_tool_result`; modify `internal/session/session.go:service.Delete` and `internal/backend/session.go` deletion paths plus backend startup lifecycle for temporary-artifact cleanup; modify shell spill helpers in `internal/agent/tools/bash.go` and `job_output.go` to preserve raw text before truncation.

**Consumes:** scope and ToolOutcome contracts; independent of gate internals after interfaces are frozen. **Produces:** toolArtifactStore and session-scoped page retrieval in both modes/local/SSH.

- [ ] Write `t.TempDir` tests for two workspace scopes with identical session IDs, two sessions in one workspace, restart lookup, UTF-8 boundaries, foreign IDs, missing artifact, malicious ID, symlink, cancellation, unwritable storage and interrupted temp-file write. Example API test:

```go
id, err := store.Put(ctx, "workspace-a/session-a", "call-a", fantasy.ToolOutcomeFailure, []byte("full diagnostics"))
if err != nil { t.Fatal(err) }
if _, _, _, err := store.Read(ctx, "workspace-a/session-b", id, 0, 16); err == nil {
    t.Fatal("foreign-session artifact was readable")
}
```

- [ ] Run `go test ./internal/agent -run TestToolArtifact -count=1`; expect missing storage/control.
- [ ] Store under `<DataDir>/tool-results/<hashed-scope>/<random-id>/` with data and manifest written atomically via temporary files/rename; manifest includes checksum, byte count, call ID and outcome. IDs are random opaque tokens, never paths. Validate canonical storage containment and refuse symlinks/reparse escape; filesystem permission failure is surfaced. Bound pages at 16384 bytes and echo next/total. Do not expose absolute paths as retrieval instructions.
- [ ] Integrate session deletion cleanup, startup removal of abandoned temp files and canceled writes; no expiry of committed artifacts while a session exists. Persist successful writes before publishing any shortened result. Keep artifact deduplication by scope/call ID/content hash to avoid repeated writes on every PrepareStep; changed content gets a new ID.
- [ ] Import complete existing shell spill outputs into this store before projecting. If shell raw-output capture cannot be saved, do not truncate it; fall back to full original output. Verify errors larger than legacy 30000-char shell limit remain fully retrievable.
- [ ] Test remote-workspace registry exposes read_tool_result even though local view is absent. Backend owns artifacts regardless whether tool execution is SSH; read control must not execute local/remote filesystem tools or change SSH profile. Run agent/backend tests using a fake Desktop MCP connection, plus delete/restart fixtures.

## Task 7: Project meaningful successful results and preserve full errors

**Files:** create `internal/agent/tool_result_projection.go`, `tool_result_projection_test.go`, `tool_result_adapters.go`; modify `internal/agent/agent.go:Run.PrepareStep`, `preparePrompt` call sites only where needed for retained history; keep `OnToolResult` full persistence unchanged; extend `internal/agent/empty_search_recovery.go` only if its metadata expectations need explicit preservation.

**Consumes:** Tasks 5–6 outcomes/artifacts, Task 3 page exemptions. **Produces:** `projectToolMessages` for final outbound copies.

- [ ] Create raw tool result fixtures for file reads, grep/search, edit diagnostics, shell output, jobs, MCP media, permission denial, hook context and unknown output. Assert source message slices/nested metadata unchanged and each outgoing tool call/result ID/order still paired. A full docs/retrieval page must never become a summary.
- [ ] Run `go test ./internal/agent -run TestToolResultProjection -count=1`; expect raw outbound text/absent policy.
- [ ] Implement deterministic adapters: read/search preserve requested data and ranges/refs/cursors; mutations return changed target/count plus diagnostics; shell includes exit/cwd and meaningful stdout/stderr; background includes job ID, running state and continuation. No extra model invocation; no generic OK replacement. Up to 8192 bytes of successful text remains inline; above that save raw content then show a UTF-8-safe preview with exact bytes/ID and `read_tool_result` continuation. Large read data remains recoverably paged rather than semantically paraphrased.
- [ ] Keep all failure/unknown text <=65536 bytes unchanged. Above 65536 bytes save full text, then return explicit failure/unknown label, original count, first/last 16384 bytes, exit/status and artifact ID. Projection must preserve error output variant and StopTurn. Storage failure retains full original result plus a storage diagnostic; no dangling ID.
- [ ] Apply after selected registry refresh and before gate coverage/accounting in each PrepareStep, to both loaded history and fantasy same-run result messages. Deep clone before changing tool text. Preserve all non-text blocks/provider metadata and hook context. Discovery uses original step ClientMetadata and is unaffected. Preserve original OnToolResult events/DB content.
- [ ] Test failed output arriving on background hidden user continuation: add a typed runtime-result part to the hidden message instead of parsing arbitrary text. Preserve raw completed output before both existing 2000-byte tail truncations (`tools/bash.go:watchBackgroundJob` and `background_wake.go:backgroundJobWakePrompt`); wake notices carry an artifact ID and semantic status, with structured raw-result storage. Normalize only that typed payload through the same projection policy before sending; keep stored continuation diagnostics complete and do not modify user-authored prose. Add storage failure and UTF-8 regression cases from Review Focus. Run `go test ./internal/agent/... -count=1`.

## Task 8: Use one prepared-request view for accounting and Desktop parity

**Files:** create `internal/agent/prepared_context.go`, `prepared_context_test.go`; modify `internal/agent/context_breakdown.go`, `agent.go` step usage estimates, `internal/backend/context_test.go`; add `internal/backend/context_on_demand_test.go`; change Desktop context UI/protocol files only if existing category fields cannot represent estimates accurately.

**Consumes:** final definitions/projected messages from Tasks 2–7. **Produces:** per-session prepared snapshot for reporting, CLI/Desktop shared counts.

- [ ] Add fixture comparing actual serialized selected cards/messages with reporter categories. Show Standard all cards, Smart controls/bootstrap then discovery, full-doc pages counted as tool results, compact results counted as transmitted bytes, not DB originals. Verify reported provider usage remains unchanged.
- [ ] Run `go test ./internal/agent -run 'TestPreparedContext|TestContextBreakdown' -count=1` and backend context tests; expect full-registry/stored-size mismatch.
- [ ] Capture immutable final prepared definition/message copy per scope+session with run/step sequence; never allow session B to overwrite A's view. `ContextBreakdown` reads it; before first run or after restart compute a labeled estimate using the same preparation helpers without executing tools, creating assistant messages or artifacts. No duplicate MCP instruction accounting. Persist raw versus estimated/provider usage distinction already present.
- [ ] Align `stepToolTokens` and message estimates with that view; cache-control metadata changes do not alter system text. Backend maps the common result unchanged. Test local Desktop and SSH backend with true/false Smart Tools across full Backend close/reopen; existing configuration merge/default stays untouched.
- [ ] Verify deleting a session cleans snapshots/artifacts, switching agents/model does not transfer documentation proof, and subagent snapshots remain separate. Run shared tests and Desktop typecheck/unit suite if any protocol/presentation file changes.

## Task 9: End-to-end request capture and release-path verification

**Files:** extend `internal/agent/smart_tool_selection_test.go`, `internal/agent/smart_tool_selection_integration_test.go` (create if absent), `internal/backend/context_on_demand_test.go`; add request fixture tests beside each affected provider adapter. No changes to system templates or installer/updater are planned.

**Consumes:** Tasks 1–8. **Produces:** evidence that behavior survives normal CLI/Desktop distribution, not an installation-local patch.

- [ ] Use local fake HTTP providers to capture outbound Chat Completions/Responses catalogs and history. Initial Standard has every permitted tool plus controls, compact descriptions and unchanged constraints; Smart has bootstrap/controls only until discovery. Documented call succeeds only after a separate request and normal permission approval. Assert complete raw result in DB/UI and compact result in next request.
- [ ] Capture Anthropic/Google fixture payloads and system prompt golden bytes. Include nested schema keywords, valid empty-required arrays, provider options and tool choices. Reject unsupported provider-executed services rather than execute them before the gate. Test all provider wrappers using their common adapters.
- [ ] Execute restart, summary, doc-version refresh, Smart toggle on existing sessionAgent, cancellation, failed docs read, tool removal and parallel-call scenarios. Add two simultaneous backend workspaces (one SSH) and cross-scope artifact attacks. No paid external API is required.
- [ ] Run:

```text
go test ./vendordeps/fantasy/... ./internal/agent/... ./internal/backend ./internal/message/... -count=1
go test -race ./vendordeps/fantasy ./internal/agent ./internal/backend -count=1
go build ./...
```

If Windows race execution needs a configured C toolchain, report that requirement and run it in the existing supported CI environment rather than claiming it passed. In `desktop`: `npm run typecheck`, `npm run test:unit`; in `desktop/src-tauri`: `cargo test --lib`. Build the desktop engine using the existing `cmd/desktop-engine` packaging path, then a native UI smoke check for approvals, full error display, result retrieval and context popover. Do not publish a release under current authorization.

- [ ] Run `git diff --check`, review scope and list modified files. Verify template/system prompt files have no changes. Review each independent slice before integration and report commands actually run, failures and environmental limitations. No commit/push until separately authorized.

## Dependency graph and independent implementation slices

Planning estimates are engineering effort, not measured runtime or promises. Nine reviewable slices total approximately 12–18 engineer-days including integration/review; work can overlap after shared interfaces are agreed.

| Slice | Approximate effort | Dependencies / parallelism |
| --- | --- | --- |
| 1 canonical schemas/validation | 1–2 days | starts immediately |
| 2 compact definitions/adapters | 1–2 days | Task 1 contract; parallel with outcomes/storage |
| 3 doc pages/provenance | 1–2 days | Task 1 contract |
| 4 gate/lifecycle/modes/hooks | 2–3 days | Tasks 1–3; owns central integration |
| 5 semantic outcomes | 1–2 days | shared field names only; independently implementable |
| 6 artifacts/retrieval | 1–2 days | shared scope/outcome contract; independently implementable |
| 7 output projection | 1–2 days | Tasks 5–6; parallel with gate after interfaces freeze |
| 8 accounting/backend parity | 1 day | final prepared-view contract from Tasks 2/4/7 |
| 9 end-to-end/release checks | 2 days | all slices |

Do not run multiple authors over `agent.go`, fantasy `agent.go`, `tool.go` or message conversion simultaneously. Assign one integration owner for those files; independent contributors implement focused files/fixtures and send exact patches for the shared touch points. Standard still has provider count-limit exposure; two additional controls can increase count. This must be communicated, not hidden by enabling Smart Tools automatically.

## Completion checklist

- [ ] All permitted domain tools are compact initially; only the two tiny controls are inline-documented.
- [ ] Same-request read-plus-execute cannot pass; later complete-doc requests can pass existing permissions.
- [ ] Schema constraints, hooks, cancellation and parallel behavior remain valid.
- [ ] Retained docs survive turns/restarts only when trusted full bytes survive; compaction/version changes close the gate.
- [ ] Full raw results remain stored; useful compact success data and complete/recoverable diagnostics reach the model.
- [ ] Local/SSH/Desktop use one policy and session-scoped retrieval with no filesystem escape.
- [ ] Reported estimates describe outgoing prepared context, not the full registry/history.
- [ ] System prompt/template bytes and Smart Tools configuration semantics remain unchanged.
