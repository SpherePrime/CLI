package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

// OrchestratorPhase represents the active phase of a multi-model Orchestrator turn.
type OrchestratorPhase string

const (
	OrchestratorPhasePlan    OrchestratorPhase = "plan"
	OrchestratorPhaseExecute OrchestratorPhase = "execute"
	OrchestratorPhaseReview  OrchestratorPhase = "review"
)

const (
	// HandoffToExecutorToolName is the virtual tool used by the Planner/Reviewer
	// to hand an approved plan to the Executor model.
	HandoffToExecutorToolName = "handoff_to_executor"
	// EscalateToPlannerToolName is the virtual tool used by the Executor to
	// return control to the Planner when blocked or when re-planning is needed.
	EscalateToPlannerToolName = "escalate_to_planner"

	maxOrchestratorEscalations        = 3
	maxOrchestratorReviewRounds       = 2
	maxOrchestratorFailureRun         = 2
	maxOrchestratorStepsBeforeCheckIn = 8
	maxOrchestratorProgressCheckIns   = 3
)

// OrchestratorRoleConfig holds the resolved model and call configuration for
// either the Planner/Reviewer role or the Executor role.
type OrchestratorRoleConfig struct {
	Model              Model
	ProviderOptions    fantasy.ProviderOptions
	SystemPromptPrefix string
	MaxOutputTokens    int64
	Temperature        *float64
	TopP               *float64
	TopK               *int64
	FrequencyPenalty   *float64
	PresencePenalty    *float64
	OnAuthRefresh      func(context.Context, *fantasy.ProviderError) error
}

// OrchestratorCallConfig configures per-turn Orchestrator mode execution on
// [SessionAgentCall].
type OrchestratorCallConfig struct {
	Enabled            bool
	FinalReviewEnabled bool
	GameDev            bool
	Planner            OrchestratorRoleConfig
	Executor           OrchestratorRoleConfig
}

type handoffToExecutorParams struct {
	Plan               string   `json:"plan" description:"Concrete step-by-step implementation plan for the executor"`
	TargetFiles        []string `json:"target_files,omitempty" description:"Target file paths to inspect or modify"`
	VerificationChecks []string `json:"verification_checks,omitempty" description:"Checks or commands the executor should run to verify the work"`
	EscalationTriggers []string `json:"escalation_triggers,omitempty" description:"Conditions under which the executor should escalate back to the planner"`
}

type escalateToPlannerParams struct {
	Reason    string `json:"reason" description:"Why execution is blocked or requires re-planning"`
	Attempted string `json:"attempted,omitempty" description:"What has been attempted so far"`
	Blocker   string `json:"blocker,omitempty" description:"Specific error, conflict, or missing information"`
}

type modelIdentity struct {
	provider string
	model    string
}

func modelIdentityOf(m Model) modelIdentity {
	p := m.ModelCfg.Provider
	id := m.ModelCfg.Model
	if p == "" && m.Model != nil {
		p = m.Model.Provider()
	}
	if id == "" && m.Model != nil {
		id = m.Model.Model()
	}
	return modelIdentity{provider: p, model: id}
}

type orchestratorRuntime struct {
	mu                      sync.Mutex
	cfg                     OrchestratorCallConfig
	phase                   OrchestratorPhase
	plan                    string
	targetFiles             []string
	verificationChecks      []string
	escalationTriggers      []string
	pendingPlanReason       string
	pendingReviewReason     string
	escalations             int
	consecutiveToolFailures int
	lastFailureDetail       string
	executeSteps            int
	executeStepsSincePlan   int
	progressCheckIns        int
	madeMutations           bool
	reviewRounds            int
	stepAssistantModels     []modelIdentity
	handoffTool             fantasy.AgentTool
	escalateTool            fantasy.AgentTool
}

func newOrchestratorRuntime(cfg OrchestratorCallConfig) *orchestratorRuntime {
	rt := &orchestratorRuntime{
		cfg:   cfg,
		phase: OrchestratorPhasePlan,
	}
	rt.handoffTool = rt.buildHandoffTool()
	rt.escalateTool = rt.buildEscalateTool()
	return rt
}

func (o *orchestratorRuntime) buildHandoffTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HandoffToExecutorToolName,
		"Hand off a concrete implementation plan from the read-only Planner/Reviewer to the Executor model. Use this only when edits, file writes, or shell commands are required. For pure questions or read-only analysis, answer directly without calling this tool.",
		func(_ context.Context, params handoffToExecutorParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Plan) == "" {
				return fantasy.NewTextErrorResponse("plan is required and must be non-empty"), nil
			}
			o.mu.Lock()
			defer o.mu.Unlock()
			if o.phase == OrchestratorPhaseExecute {
				return fantasy.NewTextErrorResponse("handoff_to_executor is only available in plan or review phase"), nil
			}
			o.plan = strings.TrimSpace(params.Plan)
			o.targetFiles = slices.Clone(params.TargetFiles)
			o.verificationChecks = slices.Clone(params.VerificationChecks)
			o.escalationTriggers = slices.Clone(params.EscalationTriggers)
			o.pendingPlanReason = ""
			o.pendingReviewReason = ""
			o.consecutiveToolFailures = 0
			o.lastFailureDetail = ""
			o.executeStepsSincePlan = 0
			o.phase = OrchestratorPhaseExecute
			return fantasy.NewTextResponse("Plan handed off to Executor. Execution phase begins on the next step."), nil
		},
	)
}

func (o *orchestratorRuntime) buildEscalateTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		EscalateToPlannerToolName,
		"Escalate back to the Planner model when blocked, when requirements are ambiguous, when repeated failures occur, or when an escalation trigger is hit.",
		func(_ context.Context, params escalateToPlannerParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Reason) == "" {
				return fantasy.NewTextErrorResponse("reason is required and must be non-empty"), nil
			}
			o.mu.Lock()
			defer o.mu.Unlock()
			if o.escalations >= maxOrchestratorEscalations {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Escalation limit reached (%d per turn). Complete what is possible or report the blocker clearly to the user.", maxOrchestratorEscalations)), nil
			}
			o.escalations++
			o.phase = OrchestratorPhasePlan
			o.consecutiveToolFailures = 0
			o.executeStepsSincePlan = 0
			o.pendingReviewReason = ""
			o.lastFailureDetail = ""
			var sb strings.Builder
			fmt.Fprintf(&sb, "Executor escalated to Planner.\nReason: %s", strings.TrimSpace(params.Reason))
			if s := strings.TrimSpace(params.Attempted); s != "" {
				fmt.Fprintf(&sb, "\nAttempted: %s", s)
			}
			if s := strings.TrimSpace(params.Blocker); s != "" {
				fmt.Fprintf(&sb, "\nBlocker: %s", s)
			}
			o.pendingPlanReason = sb.String()
			return fantasy.NewTextResponse("Escalated to Planner. Re-planning starts on the next step."), nil
		},
	)
}

func (o *orchestratorRuntime) currentPhase() OrchestratorPhase {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.phase
}

func (o *orchestratorRuntime) currentRoleConfig() OrchestratorRoleConfig {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.phase == OrchestratorPhaseExecute {
		return o.cfg.Executor
	}
	return o.cfg.Planner
}

func (o *orchestratorRuntime) onSteeringMessage() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.phase != OrchestratorPhasePlan {
		o.phase = OrchestratorPhasePlan
		o.consecutiveToolFailures = 0
		o.executeStepsSincePlan = 0
		o.pendingReviewReason = ""
		o.lastFailureDetail = ""
		o.pendingPlanReason = "The user sent a mid-turn steering instruction while execution/review was in progress. Re-evaluate the plan around the user's new instruction before making further changes."
	}
}

func isOrchestratorSubagentTool(name string) bool {
	return name == AgentToolName || name == AgentJobsToolName || name == AgentMessageToolName
}

func (o *orchestratorRuntime) filterTools(all []fantasy.AgentTool) []fantasy.AgentTool {
	phase := o.currentPhase()
	filtered := make([]fantasy.AgentTool, 0, len(all)+1)
	if phase == OrchestratorPhaseExecute {
		for _, t := range all {
			name := t.Info().Name
			if name == HandoffToExecutorToolName || name == EscalateToPlannerToolName || isOrchestratorSubagentTool(name) {
				continue
			}
			filtered = append(filtered, t)
		}
		filtered = append(filtered, o.escalateTool)
		return filtered
	}

	for _, t := range all {
		name := t.Info().Name
		if name == HandoffToExecutorToolName || name == EscalateToPlannerToolName || isOrchestratorSubagentTool(name) {
			continue
		}
		if isOrchestratorReadOnlyTool(name) {
			filtered = append(filtered, t)
		}
	}
	filtered = append(filtered, o.handoffTool)
	return filtered
}

func (o *orchestratorRuntime) beforeToolCallGate() func(context.Context, fantasy.ToolCall) error {
	return func(_ context.Context, call fantasy.ToolCall) error {
		if isOrchestratorSubagentTool(call.Name) {
			return fmt.Errorf("tool %q is disabled in orchestrator mode; execute directly instead of spawning subagents", call.Name)
		}
		phase := o.currentPhase()
		if phase == OrchestratorPhaseExecute {
			if call.Name == HandoffToExecutorToolName {
				return fmt.Errorf("tool %q is not allowed in orchestrator %s phase", call.Name, phase)
			}
			return nil
		}
		if !isOrchestratorReadOnlyTool(call.Name) && call.Name != HandoffToExecutorToolName {
			return fmt.Errorf("tool %q is disabled in orchestrator %s phase (read-only)", call.Name, phase)
		}
		return nil
	}
}

func formatOrchestratorPlanDetails(plan string, targetFiles, verificationChecks, escalationTriggers []string) string {
	var sb strings.Builder
	sb.WriteString(plan)
	if len(targetFiles) > 0 {
		sb.WriteString("\n\nTarget files:\n- ")
		sb.WriteString(strings.Join(targetFiles, "\n- "))
	}
	if len(verificationChecks) > 0 {
		sb.WriteString("\n\nVerification checks:\n- ")
		sb.WriteString(strings.Join(verificationChecks, "\n- "))
	}
	if len(escalationTriggers) > 0 {
		sb.WriteString("\n\nEscalation triggers:\n- ")
		sb.WriteString(strings.Join(escalationTriggers, "\n- "))
	}
	return sb.String()
}

func (o *orchestratorRuntime) phaseSystemPrompt() string {
	o.mu.Lock()
	defer o.mu.Unlock()

	switch o.phase {
	case OrchestratorPhaseExecute:
		var sb strings.Builder
		sb.WriteString("<orchestrator_role>You are the Orchestrator EXECUTOR phase in a single-trajectory loop. Orchestrator mode overrides any coordinator or subagent-delegation instructions in the base system prompt: subagent tools (`agent`, `agent_jobs`, `agent_message`) are disabled and blocked. You MUST implement edits, write tests, and run builds, linters, and verification checks directly yourself.\n")
		sb.WriteString("Follow the approved plan below accurately and report concrete observed results. Call `escalate_to_planner` instead of guessing if you get blocked, an escalation trigger fires, an interface mismatch appears, or verification fails repeatedly.</orchestrator_role>\n")
		sb.WriteString("<orchestrator_plan>\n")
		sb.WriteString(formatOrchestratorPlanDetails(o.plan, o.targetFiles, o.verificationChecks, o.escalationTriggers))
		sb.WriteString("\n</orchestrator_plan>")
		if o.cfg.GameDev {
			sb.WriteString("\n<gamedev_mode>\nGameDev mode is enabled (MANDATORY). Read prime://skills/prime-gamedev/SKILL.md with view. Build with solid game fundamentals upfront: concrete balance plan before coding, airtight collisions & zero floor/texture fall-throughs (matching hitboxes, fixed/clamped dt <= 0.05s, map bounds, 3D camera collision), mandatory high-contrast crosshair & combat feedback whenever any weapon/aimed attack exists, clean controls/game feel, fair enemies, and readable HUD (scoped to mod additions for third-party game mods). Query <gamedev_library> selectively via library.mjs lookup to save context (never read unrelated scripts) and use <skill_directory>/scripts/ base helpers. When textures, sprites, 3D models, 3D weapons, or character animations are needed and the user did not explicitly ask you to craft them yourself, BY DEFAULT reuse native engine/project asset libraries first (in Unreal Engine: Content/Vault packs, Starter Content / Engine Content, Fab / Quixel Megascans; Unity/Godot: Asset Store Free / AssetLib) and download from curated [Direct / API / No-Login] sources in prime://skills/prime-gamedev/references/asset-sources.md (never pulling everything from one site and never wasting turns scraping login walls) or search the web freely. Always adapt/fix downloaded scale, weapon grip pivots, PBR textures, and character animations (skeleton retargeting, foot-ground contact, root motion, blends/speed — if any animation or asset looks bad or even slightly awkward, edit/fix or replace it yourself), crafting from scratch only when requested or unavailable. Run all three QA stages: (A) place EVERY created, engine-library, AND downloaded visual element separately on a dedicated Showcase scene (characters, armed enemies, weapons & ADS sights, items, props, structures, effects, mod additions), run a fast multi-angle photo script (e.g. presets/showcase_capture_probe.mjs) AND for full games/levels also capture screenshots of the assembled map/level (overhead + gameplay-camera views) for floating/sunk props, wall/floor gaps, scale, lighting, and tiling, following prime://skills/prime-gamedev/references/game-craft.md (including real death animations, smart AI, input hygiene, audio, and Scale to Scope)) and inspect all photos with view, fixing/remaking any ugly or broken asset; (B) write and run deterministic code-level collision tests (e.g. unit/integration physics tests or presets/collision_suite_probe.mjs) verifying every solid object has a real collider matching its visual bounds and blocks movement from all 4/6 sides, corners, seams, tunneling, map bounds, and 3D camera walls — NEVER teleporting around the map; (C) run a live gameplay bot in the background without hijacking the user's mouse or keyboard (completely inside Prime Desktop's built-in browser via browser_record_start/stop + browser_run/press_key/evaluate for web games, or window-scoped capture targeting ONLY the game window via native_bot_runner.mjs — never the full computer screen) with a hard bot timeout and liveness check (verify visible state/screen changes and bot stdout/stderr; if the screen stays static/frozen or nothing happens, immediately kill the stuck process, fix the bot or input bug, and rerun), slice the video into frames/contact sheets, verify controls, combat/crosshair, runtime errors, softlocks, sustained FPS >= 50 (optimize if below 50 FPS and include real Graphics/Quality Settings such as shadow quality, Low/Medium/High presets, draw distance/effects, and render scale for standalone medium/large full games — not mods), and difficulty/balance, fix every bug found and re-verify cleanly, and save reusable scripts into sorted <gamedev_library> folders without deleting old ones.\n</gamedev_mode>")
		}
		return sb.String()

	case OrchestratorPhaseReview:
		var sb strings.Builder
		sb.WriteString("<orchestrator_role>You are the Orchestrator REVIEW phase (read-only) in a single-trajectory loop. Subagent delegation is disabled. Inspect modified files, tool outputs, and verification results against the user request and plan using read-only tools. Do not fabricate checks you did not inspect.\n")
		sb.WriteString("Choose one of two outcomes:\n")
		sb.WriteString("- If the work is complete and verified (or cannot proceed further cleanly), wrap up and deliver a concise, accurate final response directly to the user WITHOUT calling `handoff_to_executor`.\n")
		sb.WriteString("- If remaining steps, fixes, or missing verification checks are needed, call `handoff_to_executor` with a targeted follow-up plan.</orchestrator_role>")
		if o.pendingReviewReason != "" {
			sb.WriteString("\n<orchestrator_review_note>\n")
			sb.WriteString(o.pendingReviewReason)
			sb.WriteString("\n</orchestrator_review_note>")
		}
		if o.plan != "" {
			sb.WriteString("\n<orchestrator_plan>\n")
			sb.WriteString(formatOrchestratorPlanDetails(o.plan, o.targetFiles, o.verificationChecks, nil))
			sb.WriteString("\n</orchestrator_plan>")
		}
		if o.cfg.GameDev {
			sb.WriteString("\n<gamedev_mode>\nGameDev mode is enabled (MANDATORY). Read prime://skills/prime-gamedev/SKILL.md with view. Verify core game fundamentals (balance plan, airtight collisions with zero floor/texture fall-throughs, mandatory crosshair & feedback whenever weapons exist, clean HUD/restarts) and confirm all three QA stages completed cleanly without hijacking the user's mouse/keyboard: (A) a dedicated Showcase scene containing EVERY created and downloaded element (characters, weapons, props, structures, mod additions) was photographed from multiple angles via script and inspected with view with zero ugly/broken assets; (B) deterministic code-level collision tests ran and passed, verifying every solid object has a real collider matching its visual bounds and blocks movement from all 4/6 sides, corners, seams, tunneling, map bounds, and 3D camera walls without teleporting around the map; (C) a live gameplay bot ran inside Prime Desktop's built-in browser for web games when browser_* tools exist (or targeting ONLY the game window — never the full computer screen), sliced into frames and verified for controls, combat/crosshair, runtime errors, softlocks, and difficulty/balance, and every bug found was fixed and re-verified cleanly.\n</gamedev_mode>")
		}
		return sb.String()

	default:
		var sb strings.Builder
		sb.WriteString("<orchestrator_role>You are the Orchestrator PLANNER phase (read-only) in a single-trajectory loop without subagent delegation (`agent`, `agent_jobs`, `agent_message` are disabled). You only have read-only tools in this phase.\n")
		sb.WriteString("- For pure questions, explanations, or read-only analysis/review that require NO file edits, writes, or shell commands, answer directly and do NOT call `handoff_to_executor`.\n")
		sb.WriteString("- For small, clear follow-ups where scope and target files are already evident, briefly inspect only what is needed and call `handoff_to_executor` promptly.\n")
		sb.WriteString("- For non-trivial changes, thoroughly explore dependencies, callers, and affected types read-only first, then call `handoff_to_executor` with a concrete step-by-step `plan`, `target_files`, `verification_checks`, and explicit `escalation_triggers`.</orchestrator_role>")
		if o.pendingPlanReason != "" {
			if o.plan != "" {
				sb.WriteString("\n<previous_orchestrator_plan>\n")
				sb.WriteString(formatOrchestratorPlanDetails(o.plan, o.targetFiles, o.verificationChecks, o.escalationTriggers))
				sb.WriteString("\n</previous_orchestrator_plan>")
			}
			sb.WriteString("\n<orchestrator_escalation>\n")
			sb.WriteString(o.pendingPlanReason)
			sb.WriteString("\n</orchestrator_escalation>")
		}
		if o.cfg.GameDev {
			sb.WriteString("\n<gamedev_mode>\nGameDev mode is enabled (MANDATORY). Use prime://skills/prime-gamedev/SKILL.md. For any game or mod on any engine or stack, include in your plan and `verification_checks`: (1) upfront game engineering baseline (concrete balance plan, airtight collisions & zero floor/texture fall-throughs, fixed/clamped dt, mandatory crosshair & combat feedback whenever weapons exist, clean HUD/restarts — scoped to mod additions for third-party game mods), (2) selective lookup in <gamedev_library> via library.mjs (never reading unrelated folders), (3) Stage A: reusing native engine/project libraries first (in Unreal Engine: Content/Vault packs, Starter Content / Engine Content, and Fab / Quixel Megascans) and sourcing ready textures/3D models online when not requested custom (or crafting them directly when requested or unavailable) and placing EVERY created, engine-library, and downloaded element (including weapons) on a dedicated Showcase scene for fast multi-angle photo capture and view inspection, (4) Stage B: deterministic code-level collision tests verifying every solid object has a real collider matching its visual bounds and blocks movement from all 4/6 sides, corners, seams, tunneling, bounds, and 3D camera walls (never teleporting around the map), (5) Stage C: running a live gameplay bot in the background without hijacking the user's mouse/keyboard (completely in Prime Desktop's built-in browser via browser_record_start/stop + browser_run/press_key/evaluate for web games, or recording ONLY the target game window/viewport — never the full computer screen) to verify controls, combat/crosshair, runtime errors, softlocks, sustained FPS >= 50 (optimize if below 50 FPS and include real Graphics/Quality Settings such as shadow quality, Low/Medium/High presets, draw distance/effects, and render scale for standalone medium/large full games — not mods), and difficulty/balance, and (6) fixing any bugs found, re-verifying cleanly, and saving reusable scripts to <gamedev_library>.\n</gamedev_mode>")
		}
		return sb.String()
	}
}

func (o *orchestratorRuntime) reviewTriggerText() string {
	return "<orchestrator_review_request>Execution has finished. Verify the resulting changes and tool outputs against the plan using read-only tools. Either deliver the final response directly to the user, or call `handoff_to_executor` if fixes are still needed.</orchestrator_review_request>"
}

func (o *orchestratorRuntime) onStepFinish(stepPhase OrchestratorPhase, stepModel Model, stepResult fantasy.StepResult, sanitizedToolCalls map[string]bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	id := modelIdentityOf(stepModel)
	for _, m := range stepResult.Messages {
		if m.Role == fantasy.MessageRoleAssistant {
			o.stepAssistantModels = append(o.stepAssistantModels, id)
		}
	}

	if stepPhase != OrchestratorPhaseExecute {
		return
	}

	o.executeSteps++
	if o.phase == OrchestratorPhaseExecute {
		o.executeStepsSincePlan++
	}

	for _, tc := range stepResult.Content.ToolCalls() {
		if isOrchestratorMutationTool(tc.ToolName) {
			o.madeMutations = true
			break
		}
	}

	for _, tr := range stepResult.Content.ToolResults() {
		if tr.ToolName == HandoffToExecutorToolName || tr.ToolName == EscalateToPlannerToolName {
			continue
		}
		isErr := tr.Result.GetType() == fantasy.ToolResultContentTypeError ||
			tr.Outcome == fantasy.ToolOutcomeFailure ||
			sanitizedToolCalls[tr.ToolCallID]
		if !isErr {
			o.consecutiveToolFailures = 0
			o.lastFailureDetail = ""
			continue
		}
		if isOrchestratorFailureTrackedTool(tr.ToolName) {
			o.consecutiveToolFailures++
			o.lastFailureDetail = formatOrchestratorToolFailure(tr)
		}
	}

	if o.phase == OrchestratorPhaseExecute && o.consecutiveToolFailures >= maxOrchestratorFailureRun {
		if o.escalations < maxOrchestratorEscalations {
			o.escalations++
			o.phase = OrchestratorPhasePlan
			o.consecutiveToolFailures = 0
			o.executeStepsSincePlan = 0
			o.pendingReviewReason = ""
			o.pendingPlanReason = fmt.Sprintf(
				"Automatic escalation after %d consecutive tool failures in execute phase (%s). Re-evaluate the approach and produce a revised plan.",
				maxOrchestratorFailureRun,
				o.lastFailureDetail,
			)
		}
	}

	if o.phase == OrchestratorPhaseExecute &&
		stepResult.FinishReason == fantasy.FinishReasonToolCalls &&
		o.executeStepsSincePlan >= maxOrchestratorStepsBeforeCheckIn &&
		o.progressCheckIns < maxOrchestratorProgressCheckIns {
		o.phase = OrchestratorPhaseReview
		o.progressCheckIns++
		o.executeStepsSincePlan = 0
		o.pendingReviewReason = fmt.Sprintf(
			"Progress check-in (%d/%d) after %d execution steps. Inspect modified files and tool results against the plan; either wrap up directly to the user or call `handoff_to_executor` to continue.",
			o.progressCheckIns,
			maxOrchestratorProgressCheckIns,
			maxOrchestratorStepsBeforeCheckIn,
		)
	}
}

func (o *orchestratorRuntime) continueAfterStop(_ []fantasy.StepResult, _ []fantasy.Message) (bool, []fantasy.Message) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.cfg.FinalReviewEnabled || o.phase != OrchestratorPhaseExecute {
		return false, nil
	}
	if !o.madeMutations && o.executeSteps < 2 {
		return false, nil
	}
	if o.reviewRounds >= maxOrchestratorReviewRounds {
		return false, nil
	}

	o.phase = OrchestratorPhaseReview
	o.reviewRounds++
	o.consecutiveToolFailures = 0
	o.pendingReviewReason = ""
	return true, []fantasy.Message{
		fantasy.NewUserMessage(o.reviewTriggerText()),
	}
}

func (o *orchestratorRuntime) sanitizeMessages(messages []fantasy.Message, historyProducers []modelIdentity, activeModel Model) []fantasy.Message {
	o.mu.Lock()
	stepProducers := slices.Clone(o.stepAssistantModels)
	o.mu.Unlock()

	allProducers := make([]modelIdentity, 0, len(historyProducers)+len(stepProducers))
	allProducers = append(allProducers, historyProducers...)
	allProducers = append(allProducers, stepProducers...)

	return sanitizeMessagesForModel(messages, allProducers, activeModel)
}

func sanitizeMessagesForModel(messages []fantasy.Message, producers []modelIdentity, activeModel Model) []fantasy.Message {
	activeID := modelIdentityOf(activeModel)
	out := make([]fantasy.Message, 0, len(messages))
	assistantIdx := 0

	for _, msg := range messages {
		if msg.Role != fantasy.MessageRoleAssistant {
			out = append(out, msg)
			continue
		}
		var producer modelIdentity
		hasProducer := assistantIdx < len(producers)
		if hasProducer {
			producer = producers[assistantIdx]
		}
		assistantIdx++

		if hasProducer && producer == activeID {
			out = append(out, msg)
			continue
		}

		cleaned, keep := stripCrossModelAssistantParts(msg)
		if keep {
			out = append(out, cleaned)
		}
	}
	return out
}

type summarizeModelContextKey struct{}

type summarizeModelOverride struct {
	model              Model
	systemPromptPrefix string
}

func withSummarizeModel(ctx context.Context, model Model, systemPromptPrefix string) context.Context {
	return context.WithValue(ctx, summarizeModelContextKey{}, summarizeModelOverride{
		model:              model,
		systemPromptPrefix: systemPromptPrefix,
	})
}

func summarizeModelFromContext(ctx context.Context) (Model, string, bool) {
	if ctx == nil {
		return Model{}, "", false
	}
	override, ok := ctx.Value(summarizeModelContextKey{}).(summarizeModelOverride)
	if !ok || override.model.Model == nil {
		return Model{}, "", false
	}
	return override.model, override.systemPromptPrefix, true
}

func stripCrossModelAssistantParts(msg fantasy.Message) (fantasy.Message, bool) {
	newContent := make([]fantasy.MessagePart, 0, len(msg.Content))
	for _, part := range msg.Content {
		switch p := part.(type) {
		case fantasy.ReasoningPart:
			// Drop reasoning text and provider-specific signatures
			// (Anthropic signature, Google ThoughtSignature, OpenAI
			// ResponsesData) when replaying to a different model.
			continue
		case fantasy.TextPart:
			p.ProviderOptions = nil
			newContent = append(newContent, p)
		case fantasy.ToolCallPart:
			p.ProviderOptions = nil
			newContent = append(newContent, p)
		case fantasy.FilePart:
			p.ProviderOptions = nil
			newContent = append(newContent, p)
		case fantasy.ToolResultPart:
			p.ProviderOptions = nil
			newContent = append(newContent, p)
		default:
			newContent = append(newContent, part)
		}
	}
	if len(newContent) == 0 {
		return fantasy.Message{}, false
	}
	msg.Content = newContent
	msg.ProviderOptions = nil
	return msg, true
}

func setOrchestratorSystemPrompt(messages []fantasy.Message, baseSystemPrompt, phasePrompt string) []fantasy.Message {
	fullSystem := combineSystemPrompts(baseSystemPrompt, phasePrompt)
	out := slices.Clone(messages)
	if fullSystem == "" {
		return out
	}
	if len(out) > 0 && out[0].Role == fantasy.MessageRoleSystem {
		out[0] = fantasy.NewSystemMessage(fullSystem)
		return out
	}
	return append([]fantasy.Message{fantasy.NewSystemMessage(fullSystem)}, out...)
}

func combineSystemPrompts(base, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	switch {
	case base == "":
		return extra
	case extra == "":
		return base
	default:
		return base + "\n\n" + extra
	}
}

func collectHistoryAssistantModels(msgs []message.Message) []modelIdentity {
	var out []modelIdentity
	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		if m.Role == message.Tool {
			continue
		}
		if m.Role == message.Assistant {
			out = append(out, modelIdentity{provider: m.Provider, model: m.Model})
		}
	}
	return out
}

func isOrchestratorReadOnlyTool(name string) bool {
	switch name {
	case tools.ViewToolName,
		tools.GlobToolName,
		tools.GrepToolName,
		tools.LSToolName,
		tools.SymbolsToolName,
		tools.DiagnosticsToolName,
		tools.DefinitionToolName,
		tools.ReferencesToolName,
		tools.CallHierarchyToolName,
		tools.SourcegraphToolName,
		tools.QuestionToolName,
		tools.SearchInstructionsToolName,
		tools.ReadInstructionToolName,
		tools.WebSearchToolName,
		tools.WebFetchToolName,
		tools.FetchToolName,
		tools.AgenticFetchToolName,
		tools.TodosToolName,
		tools.PrimeInfoToolName,
		tools.PrimeLogsToolName,
		"search_skills",
		"search_tools",
		"search_mcp",
		tools.ListMCPResourcesToolName,
		tools.ReadMCPResourceToolName,
		readToolDocumentationName,
		readToolResultName,
		HandoffToExecutorToolName:
		return true
	default:
		return false
	}
}

func isOrchestratorMutationTool(name string) bool {
	if name == HandoffToExecutorToolName || name == EscalateToPlannerToolName {
		return false
	}
	switch name {
	case tools.WriteToolName,
		tools.EditToolName,
		tools.MultiEditToolName,
		tools.BashToolName,
		tools.SudoToolName,
		tools.RenameToolName,
		tools.ReplaceSymbolToolName,
		tools.ImageGenToolName,
		tools.DownloadToolName,
		AgentToolName:
		return true
	default:
		return !isOrchestratorReadOnlyTool(name)
	}
}

func isOrchestratorFailureTrackedTool(name string) bool {
	if name == HandoffToExecutorToolName || name == EscalateToPlannerToolName {
		return false
	}
	return !isOrchestratorReadOnlyTool(name)
}

func formatOrchestratorToolFailure(tr fantasy.ToolResultContent) string {
	errMsg := "tool error"
	if e, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](tr.Result); ok && e.Error != nil {
		errMsg = strings.TrimSpace(e.Error.Error())
	}
	if len(errMsg) > 160 {
		errMsg = errMsg[:160] + "..."
	}
	return fmt.Sprintf("last tool %q: %s", tr.ToolName, errMsg)
}
