package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/hyper"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/agent/prompt"
	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/customagents"
	"github.com/SpherePrime/CLI/internal/discover"
	"github.com/SpherePrime/CLI/internal/event"
	"github.com/SpherePrime/CLI/internal/filetracker"
	"github.com/SpherePrime/CLI/internal/history"
	"github.com/SpherePrime/CLI/internal/log"
	"github.com/SpherePrime/CLI/internal/oauth"
	"github.com/SpherePrime/CLI/internal/oauth/copilot"
	openaioauth "github.com/SpherePrime/CLI/internal/oauth/openai"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/azure"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/bedrock"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/google"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openaicompat"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openrouter"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/vercel"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	openaisdk "github.com/SpherePrime/CLI/vendordeps/openai/openai-go/v3/option"
)

type CustomAgentRuntime = CustomExecutor

type CustomExecutorOption func(*CustomExecutor)

func WithExecutorService(svc *customagents.Service) CustomExecutorOption {
	return func(e *CustomExecutor) {
		e.svc = svc
	}
}

func WithAccountService(acct *account.Service) CustomExecutorOption {
	return func(e *CustomExecutor) {
		e.acct = acct
	}
}

type CustomExecutor struct {
	cfg  *config.ConfigStore
	acct *account.Service
	mu   sync.RWMutex
	svc  *customagents.Service
}

func NewCustomExecutor(cfgStore *config.ConfigStore, opts ...CustomExecutorOption) *CustomExecutor {
	exec := &CustomExecutor{
		cfg: cfgStore,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(exec)
		}
	}
	return exec
}

func (e *CustomExecutor) SetService(svc *customagents.Service) {
	e.mu.Lock()
	e.svc = svc
	e.mu.Unlock()
}

func (e *CustomExecutor) service() *customagents.Service {
	e.mu.RLock()
	svc := e.svc
	e.mu.RUnlock()
	return svc
}

func (e *CustomExecutor) accountToken() string {
	if e.acct != nil {
		return e.acct.Token()
	}
	svc := account.NewService()
	defer svc.Close()
	return svc.Token()
}

func (e *CustomExecutor) ListAvailableModels(ctx context.Context) ([]customagents.ModelOption, error) {
	if e.cfg == nil {
		return nil, nil
	}
	cfg := e.cfg.Config()
	if cfg == nil {
		return nil, nil
	}
	if cfg.Options != nil && !cfg.Options.DisableDefaultProviders {
		if token := e.accountToken(); token != "" {
			refreshCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
			if p, ok := cfg.Providers.Get(primeai.Name); !ok || (!p.Disable && len(p.Models) <= 1) {
				_ = e.cfg.EnsurePrimeAI(refreshCtx)
			} else if !p.Disable {
				_ = e.cfg.RefreshPrimeAIIfStale(refreshCtx, 20*time.Second)
			}
			cancel()
			cfg = e.cfg.Config()
		}
	}

	var options []customagents.ModelOption
	for providerID, provider := range cfg.Providers.Seq2() {
		if provider.Disable {
			continue
		}
		providerModels := provider.Models
		if provider.OAuthToken != nil && len(provider.ChatGPTModels) > 0 {
			providerModels = provider.ChatGPTModels
		}
		for _, model := range providerModels {
			if config.IsImageOutputModel(model) {
				continue
			}
			var planLabel string
			var locked bool
			if providerID == primeai.Name {
				if access, ok := e.cfg.PrimeAIModelAccess(model.ID); ok {
					planLabel = access.PlanLabel
					locked = access.Locked
				}
			}
			modes := []string{config.ReasoningModeOff}
			if model.CanReason {
				modes = append(modes, config.AllReasoningModes()...)
			}
			name := model.Name
			if name == "" {
				name = model.ID
			}
			options = append(options, customagents.ModelOption{
				ProviderID:           providerID,
				ProviderName:         provider.Name,
				ModelID:              model.ID,
				Name:                 name,
				Tier:                 classifyModelTier(model),
				CanReason:            model.CanReason,
				ReasoningModes:       modes,
				ReasoningLevels:      slices.Clone(model.ReasoningLevels),
				DefaultReasoningMode: config.SelectedModel{}.ResolveReasoningMode(model),
				ContextWindow:        model.ContextWindow,
				CostPer1MIn:          model.CostPer1MIn,
				CostPer1MOut:         model.CostPer1MOut,
				Locked:               locked,
				PlanLabel:            planLabel,
			})
		}
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].ProviderID+"/"+options[i].ModelID < options[j].ProviderID+"/"+options[j].ModelID
	})
	return options, nil
}

func classifyModelTier(model catwalk.Model) string {
	id := strings.ToLower(model.ID + " " + model.Name)
	fastTokens := []string{"mini", "flash", "haiku", "nano", "lite", "small", "fast", "instant", "turbo", "8b", "7b", "3b"}
	flagshipTokens := []string{"opus", "o3", "o4", "gpt-5", "r1", "ultra", "flagship", "max"}
	reasoningTokens := []string{"sonnet", "pro", "reason", "thinking", "deep", "coder"}

	for _, tok := range fastTokens {
		if hasBoundaryToken(id, tok) {
			return customagents.TierFast
		}
	}
	for _, tok := range flagshipTokens {
		if hasBoundaryToken(id, tok) {
			return customagents.TierFlagship
		}
	}
	for _, tok := range reasoningTokens {
		if hasBoundaryToken(id, tok) {
			return customagents.TierReasoning
		}
	}
	if model.CanReason {
		if model.CostPer1MIn >= 10 {
			return customagents.TierFlagship
		}
		if model.CostPer1MIn > 0 && model.CostPer1MIn <= 0.6 {
			return customagents.TierFast
		}
		return customagents.TierReasoning
	}
	if model.CostPer1MIn > 0 && model.CostPer1MIn <= 0.6 {
		return customagents.TierFast
	}
	if model.CostPer1MIn >= 10 {
		return customagents.TierFlagship
	}
	return customagents.TierBalanced
}

func hasBoundaryToken(text, tok string) bool {
	if tok == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(text[start:], tok)
		if idx < 0 {
			return false
		}
		pos := start + idx
		leftOK := pos == 0 || !isAlphaNumASCII(text[pos-1])
		rightPos := pos + len(tok)
		rightOK := rightPos == len(text) || !isAlphaNumASCII(text[rightPos])
		if leftOK && rightOK {
			return true
		}
		start = pos + 1
	}
}

func isAlphaNumASCII(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

func (e *CustomExecutor) Execute(ctx context.Context, req customagents.ExecuteRequest) (customagents.ExecuteResult, error) {
	if e.cfg == nil {
		return customagents.ExecuteResult{}, errors.New("config store is not configured")
	}
	cfg := e.cfg.Config()
	if cfg == nil {
		return customagents.ExecuteResult{}, errors.New("config is not available")
	}
	providerCfg, ok := cfg.Providers.Get(req.ProviderID)
	if !ok || providerCfg.Disable {
		return customagents.ExecuteResult{}, fmt.Errorf("provider %q is not configured or disabled", req.ProviderID)
	}

	providerModels := providerCfg.Models
	if providerCfg.OAuthToken != nil && len(providerCfg.ChatGPTModels) > 0 {
		providerModels = providerCfg.ChatGPTModels
	}
	var catalogModel *catwalk.Model
	for i := range providerModels {
		if providerModels[i].ID == req.ModelID {
			catalogModel = &providerModels[i]
			break
		}
	}
	if catalogModel == nil {
		return customagents.ExecuteResult{}, fmt.Errorf("model %q not found on provider %q", req.ModelID, req.ProviderID)
	}

	selected := config.SelectedModel{
		Provider:      req.ProviderID,
		Model:         req.ModelID,
		ReasoningMode: req.ReasoningMode,
		MaxTokens:     catalogModel.DefaultMaxTokens,
	}
	provider, err := buildProviderForConfig(e.cfg, providerCfg, selected, true)
	if err != nil {
		return customagents.ExecuteResult{}, err
	}

	modelID := req.ModelID
	if req.ProviderID == openrouter.Name && isExactoSupported(modelID) {
		modelID += ":exacto"
	}
	languageModel, err := provider.LanguageModel(ctx, modelID)
	if err != nil {
		return customagents.ExecuteResult{}, err
	}

	requestTimeout := cfg.Options.GetRequestTimeout()
	languageModel = newRequestTimeoutModel(languageModel, requestTimeout)
	languageModel = newReasoningCompatModel(newToolRoleCompatModel(newToolCallLeakModel(languageModel)))

	model := Model{
		Model:      languageModel,
		CatwalkCfg: applyModelOverrides(*catalogModel, selected),
		ModelCfg:   selected,
		FlatRate:   providerCfg.FlatRate,
	}

	mergedOptions, temp, topP, topK, freqPenalty, presPenalty := mergeCallOptions(model, providerCfg)
	maxTokens := callMaxOutputTokens(model, providerCfg.Type)
	var maxOutputTokens *int64
	if maxTokens > 0 {
		maxOutputTokens = &maxTokens
	}
	effectiveTopK := callTopK(providerCfg, topK)

	workDir := strings.TrimSpace(req.Agent.WorkspacePath)
	if workDir == "" {
		workDir = e.cfg.WorkingDir()
	}
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	perms := permission.NewPermissionService(workDir, true, nil)
	ft := newCustomFileTracker()
	hist := newCustomHistoryService()
	defer hist.Broker.Shutdown()

	if strings.TrimSpace(req.MemorySnapshot) == "" {
		if svc := e.service(); svc != nil && svc.GlobalMemory() != nil {
			if doc, docErr := svc.GlobalMemory().GetDoc(ctx); docErr == nil && (strings.TrimSpace(doc.Memory) != "" || strings.TrimSpace(doc.UserProfile) != "") {
				memBlock, _ := svc.GlobalMemory().FormatFrozenSnapshot(ctx)
				req.MemorySnapshot = memBlock
			}
		}
	}

	agentTools := e.buildCustomAgentTools(req, model, workDir, perms, hist, ft)
	sysPrompt := buildCustomAgentSystemPrompt(req, workDir)

	sessionKey := req.RunID
	if sessionKey == "" {
		sessionKey = req.Agent.ID
	}
	if sessionKey == "" {
		sessionKey = uuid.NewString()
	}

	execCtx := context.WithValue(ctx, tools.SessionIDContextKey, sessionKey)
	execCtx = context.WithValue(execCtx, tools.MessageIDContextKey, sessionKey)
	execCtx = context.WithValue(execCtx, tools.SupportsImagesContextKey, model.CatwalkCfg.SupportsImages)
	execCtx = context.WithValue(execCtx, tools.ModelNameContextKey, model.CatwalkCfg.Name)

	fa := fantasy.NewAgent(
		model.Model,
		fantasy.WithSystemPrompt(sysPrompt),
		fantasy.WithTools(agentTools...),
		fantasy.WithUserAgent(userAgent),
		fantasy.WithRepairToolCall(repairToolCall),
	)

	var (
		stepMu        sync.Mutex
		recordedSteps []customagents.RunStep
	)
	emitStep := func(step customagents.RunStep) {
		if step.Timestamp == 0 {
			step.Timestamp = time.Now().Unix()
		}
		if req.OnStep != nil {
			req.OnStep(step)
			return
		}
		stepMu.Lock()
		recordedSteps = append(recordedSteps, step)
		stepMu.Unlock()
	}

	result, err := fa.Stream(execCtx, fantasy.AgentStreamCall{
		Prompt:           req.Prompt,
		ProviderOptions:  mergedOptions,
		MaxOutputTokens:  maxOutputTokens,
		Temperature:      temp,
		TopP:             topP,
		TopK:             effectiveTopK,
		FrequencyPenalty: freqPenalty,
		PresencePenalty:  presPenalty,
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			emitStep(customagents.RunStep{
				Kind:      "tool_call",
				Title:     tc.ToolName,
				Detail:    tc.Input,
				Status:    "running",
				Timestamp: time.Now().Unix(),
			})
			return nil
		},
		OnToolResult: func(tr fantasy.ToolResultContent) error {
			status := "ok"
			detail := ""
			switch tr.Result.GetType() {
			case fantasy.ToolResultContentTypeText:
				if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Result); ok {
					detail = r.Text
				}
			case fantasy.ToolResultContentTypeError:
				status = "failed"
				if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](tr.Result); ok && r.Error != nil {
					detail = r.Error.Error()
				}
			}
			if len(detail) > 1000 {
				cut := 1000
				for cut > 0 && !utf8.RuneStart(detail[cut]) {
					cut--
				}
				detail = detail[:cut] + "..."
			}
			emitStep(customagents.RunStep{
				Kind:      "tool_result",
				Title:     tr.ToolName,
				Detail:    detail,
				Status:    status,
				Timestamp: time.Now().Unix(),
			})
			return nil
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			emitStep(customagents.RunStep{
				Kind:      "llm_step",
				Title:     "Step completed",
				Detail:    string(stepResult.FinishReason),
				Status:    "ok",
				Timestamp: time.Now().Unix(),
			})
			return nil
		},
	})
	stepMu.Lock()
	stepsSnapshot := slices.Clone(recordedSteps)
	stepMu.Unlock()
	if err != nil {
		return customagents.ExecuteResult{Steps: stepsSnapshot}, err
	}

	outputText := extractCustomAgentOutput(result)
	return customagents.ExecuteResult{
		OutputText: outputText,
		Steps:      stepsSnapshot,
	}, nil
}

func extractCustomAgentOutput(result *fantasy.AgentResult) string {
	if result == nil {
		return ""
	}
	if text := result.Response.Content.Text(); strings.TrimSpace(text) != "" {
		return text
	}
	for i := len(result.Steps) - 1; i >= 0; i-- {
		if text := result.Steps[i].Response.Content.Text(); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func buildCustomAgentSystemPrompt(req customagents.ExecuteRequest, workDir string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(req.Agent.SystemPrompt))
	if trimmedMem := strings.TrimSpace(req.MemorySnapshot); trimmedMem != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if strings.Contains(trimmedMem, "<agent_memory>") || strings.Contains(trimmedMem, "<prime_memory>") || strings.Contains(trimmedMem, "<soul>") {
			b.WriteString(trimmedMem)
		} else {
			b.WriteString("<agent_memory>\n")
			b.WriteString(req.MemorySnapshot)
			b.WriteString("\n</agent_memory>")
		}
	}
	if req.Agent.StrictVerify && req.Agent.Role != customagents.RoleVerifier {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("<execution_contract>\nStrict verification is enabled: cite verified file paths, command outcomes, and concrete evidence only.\n</execution_contract>")
	}
	if strings.TrimSpace(req.BlackboardContext) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("<blackboard_context>\n")
		b.WriteString(req.BlackboardContext)
		b.WriteString("\n</blackboard_context>")
	}
	if strings.TrimSpace(workDir) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("<working_directory>\n")
		b.WriteString(workDir)
		b.WriteString("\n</working_directory>")
	}
	return b.String()
}

func (e *CustomExecutor) buildCustomAgentTools(
	req customagents.ExecuteRequest,
	model Model,
	workDir string,
	perms permission.Service,
	hist history.Service,
	ft filetracker.Service,
) []fantasy.AgentTool {
	cfg := e.cfg.Config()
	dataDir := ""
	var attribution *config.Attribution
	var globCfg config.ToolGlob
	var grepCfg config.ToolGrep
	var lsCfg config.ToolLs
	var skillsPaths []string
	if cfg != nil && cfg.Options != nil {
		dataDir = cfg.Options.DataDirectory
		attribution = cfg.Options.Attribution
		skillsPaths = cfg.Options.SkillsPaths
	}
	if cfg != nil {
		globCfg = cfg.Tools.Glob
		grepCfg = cfg.Tools.Grep
		lsCfg = cfg.Tools.Ls
	}

	standard := []fantasy.AgentTool{
		tools.NewBashTool(perms, workDir, dataDir, attribution, req.ModelID, nil),
		tools.NewViewTool(nil, perms, ft, nil, workDir, skillsPaths...),
		tools.NewEditTool(nil, perms, hist, ft, workDir),
		tools.NewMultiEditTool(nil, perms, hist, ft, workDir),
		tools.NewWriteTool(nil, perms, hist, ft, workDir),
		tools.NewGlobTool(workDir, globCfg),
		tools.NewGrepTool(workDir, grepCfg),
		tools.NewLsTool(perms, workDir, lsCfg),
		tools.NewFetchTool(perms, workDir, nil),
		e.newCustomAgenticFetchTool(model, dataDir, perms, ft, globCfg, grepCfg),
		tools.NewWebSearchTool(nil, tools.WebSearchOptions{
			Permissions:   perms,
			WorkingDir:    workDir,
			Resolver:      e.cfg.Resolver(),
			ProviderModel: func() (config.ProviderWebSearchModel, bool) { return e.cfg.Config().ProviderWebSearchModel() },
			SubAgent:      true,
		}),
		tools.NewDownloadTool(perms, workDir, nil),
		tools.NewSourcegraphTool(nil),
	}

	disallowed := map[string]bool{
		AgentToolName:         true,
		AgentJobsToolName:     true,
		AgentMessageToolName:  true,
		"goal_complete":       true,
		tools.QuestionToolName: true,
	}

	var allowedSet map[string]bool
	if len(req.Agent.AllowedTools) > 0 {
		allowedSet = make(map[string]bool, len(req.Agent.AllowedTools))
		for _, name := range req.Agent.AllowedTools {
			trimmed := strings.TrimSpace(name)
			if trimmed != "" {
				allowedSet[trimmed] = true
			}
		}
	}

	out := make([]fantasy.AgentTool, 0, len(standard)+8)
	for _, tool := range standard {
		name := tool.Info().Name
		if disallowed[name] {
			continue
		}
		if allowedSet != nil && !allowedSet[name] {
			continue
		}
		out = append(out, tool)
	}

	if svc := e.service(); svc != nil {
		out = append(out, NewCustomAgentRoleTools(svc, req.Agent, req.ChainID, req.RunID)...)
	}

	return out
}

func (e *CustomExecutor) newCustomAgenticFetchTool(
	model Model,
	dataDir string,
	perms permission.Service,
	ft filetracker.Service,
	globCfg config.ToolGlob,
	grepCfg config.ToolGrep,
) fantasy.AgentTool {
	return e.newCustomAgenticFetchToolWithTimeout(model, dataDir, perms, ft, globCfg, grepCfg, agenticFetchTimeout)
}

func (e *CustomExecutor) newCustomAgenticFetchToolWithTimeout(
	model Model,
	dataDir string,
	perms permission.Service,
	ft filetracker.Service,
	globCfg config.ToolGlob,
	grepCfg config.ToolGrep,
	timeout time.Duration,
) fantasy.AgentTool {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	return fantasy.NewParallelAgentTool(
		tools.AgenticFetchToolName,
		agenticFetchToolDescription,
		func(ctx context.Context, params tools.AgenticFetchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}
			fetchCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			tmpBase := dataDir
			if tmpBase == "" {
				tmpBase = os.TempDir()
			}
			tmpDir, err := os.MkdirTemp(tmpBase, "prime-fetch-*")
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to create temporary directory: %s", err)), nil
			}
			defer os.RemoveAll(tmpDir)

			var fullPrompt string
			if params.URL != "" {
				content, fetchErr := tools.FetchURLAndConvert(fetchCtx, client, params.URL)
				if fetchErr != nil {
					if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("agentic_fetch timed out after %s while fetching %s", timeout, params.URL)), nil
					}
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to fetch URL: %s", fetchErr)), nil
				}
				if len(content) > tools.LargeContentThreshold {
					tempFile, tfErr := os.CreateTemp(tmpDir, "page-*.md")
					if tfErr != nil {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to create temporary file: %s", tfErr)), nil
					}
					tempFilePath := tempFile.Name()
					if _, wErr := tempFile.WriteString(content); wErr != nil {
						tempFile.Close()
						return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to write content to file: %s", wErr)), nil
					}
					tempFile.Close()
					fullPrompt = fmt.Sprintf("%s\n\nThe web page from %s has been saved to: %s\n\nUse the view and grep tools to analyze this file and extract the requested information.", params.Prompt, params.URL, tempFilePath)
				} else {
					fullPrompt = fmt.Sprintf("%s\n\nWeb page URL: %s\n\n<webpage_content>\n%s\n</webpage_content>", params.Prompt, params.URL, content)
				}
			} else {
				fullPrompt = fmt.Sprintf("%s\n\nUse the web_search tool to find relevant information. Break down the question into smaller, focused searches if needed. After searching, use web_fetch to get detailed content from the most relevant results.", params.Prompt)
			}

			promptTemplate, err := prompt.NewPrompt("agentic_fetch", string(agenticFetchPromptTmpl), prompt.WithWorkingDir(tmpDir))
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating prompt: %s", err)
			}
			systemPrompt, err := promptTemplate.Build(fetchCtx, model.Model.Provider(), model.Model.Model(), e.cfg)
			if err != nil {
				if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("agentic_fetch timed out after %s", timeout)), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("error building system prompt: %s", err)
			}

			fetchTools := []fantasy.AgentTool{
				tools.NewWebFetchTool(tmpDir, client),
				tools.NewWebSearchTool(client, tools.WebSearchOptions{
					Resolver:      e.cfg.Resolver(),
					ProviderModel: func() (config.ProviderWebSearchModel, bool) { return e.cfg.Config().ProviderWebSearchModel() },
					SubAgent:      true,
					Permissions:   nil,
				}),
				tools.NewGlobTool(tmpDir, globCfg),
				tools.NewGrepTool(tmpDir, grepCfg),
				tools.NewSourcegraphTool(client),
				tools.NewViewTool(nil, perms, ft, nil, tmpDir),
			}

			fetchAgent := fantasy.NewAgent(
				model.Model,
				fantasy.WithSystemPrompt(systemPrompt),
				fantasy.WithTools(fetchTools...),
				fantasy.WithUserAgent(userAgent),
				fantasy.WithRepairToolCall(repairToolCall),
			)
			res, err := fetchAgent.Generate(fetchCtx, fantasy.AgentCall{
				Prompt:   fullPrompt,
				StopWhen: []fantasy.StopCondition{fantasy.StepCountIs(agenticFetchMaxSteps)},
			})
			if err != nil {
				if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("agentic_fetch timed out after %s", timeout)), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to generate response: %s", err)), nil
			}
			text := extractCustomAgentOutput(res)
			if text == "" {
				return fantasy.NewTextErrorResponse("agentic_fetch produced no output"), nil
			}
			return fantasy.NewTextResponse(text), nil
		},
	)
}

func buildProviderForConfig(cfgStore *config.ConfigStore, providerCfg config.ProviderConfig, model config.SelectedModel, isSubAgent bool) (fantasy.Provider, error) {
	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	if providerCfg.Type == anthropic.Name && isAnthropicThinkingModel(model) {
		if v, ok := headers["anthropic-beta"]; ok {
			headers["anthropic-beta"] = v + ",interleaved-thinking-2025-05-14"
		} else {
			headers["anthropic-beta"] = "interleaved-thinking-2025-05-14"
		}
	}

	apiKey, _ := cfgStore.Resolve(providerCfg.APIKey)
	baseURL, _ := cfgStore.Resolve(providerCfg.BaseURL)

	switch providerCfg.ID {
	case string(catwalk.InferenceProviderOpenCodeGo), string(catwalk.InferenceProviderOpenCodeZen):
		if apiKey == "" {
			apiKey = "public"
		}
		if isOpenCodeMessagesModel(providerCfg.ID, model.Model) {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
			headers["User-Agent"] = "opencode/prime"
			return buildAnthropicProviderForConfig(cfgStore, baseURL, apiKey, headers, providerCfg.ID)
		}
		headers["User-Agent"] = "opencode/prime"
	}

	switch providerCfg.Type {
	case openai.Name:
		token := providerCfg.OAuthToken
		if token != nil {
			baseURL = openaioauth.CodexBaseURL
			apiKey = token.AccessToken
			headers["originator"] = "prime"
			if token.AccountID != "" {
				headers["chatgpt-account-id"] = token.AccountID
			}
		}
		return buildOpenaiProviderForConfig(cfgStore, baseURL, apiKey, headers, token)
	case anthropic.Name:
		return buildAnthropicProviderForConfig(cfgStore, baseURL, apiKey, headers, providerCfg.ID)
	case openrouter.Name:
		return buildOpenrouterProviderForConfig(cfgStore, baseURL, apiKey, headers)
	case vercel.Name:
		return buildVercelProviderForConfig(cfgStore, baseURL, apiKey, headers)
	case azure.Name:
		return buildAzureProviderForConfig(cfgStore, baseURL, apiKey, headers, providerCfg.ExtraParams)
	case bedrock.Name:
		return buildBedrockProviderForConfig(cfgStore, apiKey, headers, providerCfg.ID)
	case google.Name:
		return buildGoogleProviderForConfig(cfgStore, baseURL, apiKey, headers)
	case "google-vertex":
		return buildGoogleVertexProviderForConfig(cfgStore, headers, providerCfg.ExtraParams)
	case primeai.Name:
		return primeai.NewProvider(primeai.WithProviderBaseURL(baseURL))
	case openaicompat.Name, hyper.Name:
		switch providerCfg.ID {
		case hyper.Name:
			baseURL = hyper.BaseURL() + "/v1"
			headers["x-prime-id"] = event.GetID()
		case string(catwalk.InferenceProviderZAI):
			if providerCfg.ExtraBody == nil {
				providerCfg.ExtraBody = map[string]any{}
			}
			providerCfg.ExtraBody["tool_stream"] = true
		}
		return buildOpenaiCompatProviderForConfig(cfgStore, baseURL, apiKey, headers, providerCfg.ExtraBody, providerCfg.ID, isSubAgent)
	default:
		if discover.IsKnownCustomProvider(string(providerCfg.Type)) {
			return buildOpenaiCompatProviderForConfig(cfgStore, baseURL, apiKey, headers, providerCfg.ExtraBody, providerCfg.ID, isSubAgent)
		}
		return nil, fmt.Errorf("provider type not supported: %q", providerCfg.Type)
	}
}

func isAnthropicThinkingModel(model config.SelectedModel) bool {
	if model.HasExplicitReasoningMode() {
		return model.IsReasoningEnabled()
	}
	opts, err := anthropic.ParseOptions(model.ProviderOptions)
	return err == nil && opts.Thinking != nil
}

func buildAnthropicProviderForConfig(cfgStore *config.ConfigStore, baseURL, apiKey string, headers map[string]string, providerID string) (fantasy.Provider, error) {
	var opts []anthropic.Option
	switch {
	case strings.HasPrefix(apiKey, "Bearer "):
		os.Setenv("ANTHROPIC_API_KEY", "")
		headers["Authorization"] = apiKey
	case providerID == string(catwalk.InferenceProviderMiniMax) || providerID == string(catwalk.InferenceProviderMiniMaxChina):
		os.Setenv("ANTHROPIC_API_KEY", "")
		headers["Authorization"] = "Bearer " + apiKey
	case apiKey != "":
		opts = append(opts, anthropic.WithAPIKey(apiKey))
	}
	if len(headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
	}
	return anthropic.New(opts...)
}

func buildOpenaiProviderForConfig(cfgStore *config.ConfigStore, baseURL, apiKey string, headers map[string]string, token *oauth.Token) (fantasy.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithUseResponsesAPI(),
	}
	var httpClient *http.Client
	if cfgStore.Config().Options.Debug {
		httpClient = log.NewHTTPClient()
	}
	if token != nil {
		if httpClient == nil {
			httpClient = &http.Client{}
		}
		httpClient.Transport = &openaioauth.Transport{
			Base:  httpClient.Transport,
			Token: token,
		}
	}
	opts = append(opts, openai.WithHTTPClient(withStreamActivityTransport(httpClient)))
	if len(headers) > 0 {
		opts = append(opts, openai.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}

func buildOpenrouterProviderForConfig(cfgStore *config.ConfigStore, _, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openrouter.Option{
		openrouter.WithAPIKey(apiKey),
	}
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openrouter.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openrouter.WithHeaders(headers))
	}
	return openrouter.New(opts...)
}

func buildVercelProviderForConfig(cfgStore *config.ConfigStore, _, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []vercel.Option{
		vercel.WithAPIKey(apiKey),
	}
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, vercel.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, vercel.WithHeaders(headers))
	}
	return vercel.New(opts...)
}

func buildOpenaiCompatProviderForConfig(cfgStore *config.ConfigStore, baseURL, apiKey string, headers map[string]string, extraBody map[string]any, providerID string, isSubAgent bool) (fantasy.Provider, error) {
	opts := []openaicompat.Option{
		openaicompat.WithBaseURL(baseURL),
		openaicompat.WithAPIKey(apiKey),
	}
	var httpClient *http.Client
	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		opts = append(
			opts,
			openaicompat.WithUseResponsesAPI(),
			openaicompat.WithResponsesAPIFunc(func(modelID string) bool {
				return copilotResponsesModels[modelID]
			}),
		)
		httpClient = copilot.NewClient(isSubAgent, cfgStore.Config().Options.Debug)
	case string(catwalk.InferenceProviderOpenCodeGo), string(catwalk.InferenceProviderOpenCodeZen):
		opts = append(
			opts,
			openaicompat.WithUseResponsesAPI(),
			openaicompat.WithResponsesAPIFunc(isOpenCodeResponsesModel),
		)
		if apiKey == "" {
			apiKey = "public"
		}
		opts = append(opts, openaicompat.WithUserAgent("opencode/prime"))
	case hyper.Name:
		opts = append(
			opts,
			openaicompat.WithLanguageModelOptions(
				openai.WithLanguageModelHeaderFunc(hyper.HeaderFunc),
			),
		)
	}
	if httpClient == nil && cfgStore.Config().Options.Debug {
		httpClient = log.NewHTTPClient()
	}
	if providerCfg, ok := cfgStore.Config().Providers.Get(providerID); ok {
		httpClient = withGatewayCapabilities(httpClient, providerCfg.Models)
	}
	opts = append(opts, openaicompat.WithHTTPClient(withStreamActivityTransport(httpClient)))
	if len(headers) > 0 {
		opts = append(opts, openaicompat.WithHeaders(headers))
	}
	for extraKey, extraValue := range extraBody {
		opts = append(opts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(extraKey, extraValue)))
	}
	return openaicompat.New(opts...)
}

func buildAzureProviderForConfig(cfgStore *config.ConfigStore, baseURL, apiKey string, headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []azure.Option{
		azure.WithBaseURL(baseURL),
		azure.WithAPIKey(apiKey),
		azure.WithUseResponsesAPI(),
	}
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, azure.WithHTTPClient(httpClient))
	}
	if options == nil {
		options = make(map[string]string)
	}
	if apiVersion, ok := options["apiVersion"]; ok {
		opts = append(opts, azure.WithAPIVersion(apiVersion))
	}
	if len(headers) > 0 {
		opts = append(opts, azure.WithHeaders(headers))
	}
	return azure.New(opts...)
}

func buildBedrockProviderForConfig(cfgStore *config.ConfigStore, apiKey string, headers map[string]string, providerID string) (fantasy.Provider, error) {
	var opts []bedrock.Option
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, bedrock.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, bedrock.WithHeaders(headers))
	}
	switch {
	case apiKey != "":
		opts = append(opts, bedrock.WithAPIKey(apiKey))
	case os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "":
		opts = append(opts, bedrock.WithAPIKey(os.Getenv("AWS_BEARER_TOKEN_BEDROCK")))
	}
	switch providerID {
	case string(catwalk.InferenceProviderBedrockEurope):
		opts = append(opts, bedrock.WithRegion("eu-west-1"))
	default:
		opts = append(opts, bedrock.WithRegion("us-east-1"))
	}
	return bedrock.New(opts...)
}

func buildGoogleProviderForConfig(cfgStore *config.ConfigStore, baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{
		google.WithBaseURL(baseURL),
		google.WithGeminiAPIKey(apiKey),
	}
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	return google.New(opts...)
}

func buildGoogleVertexProviderForConfig(cfgStore *config.ConfigStore, headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	var opts []google.Option
	if cfgStore.Config().Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	opts = append(opts, google.WithVertex(options["project"], options["location"]))
	return google.New(opts...)
}

type customFileTracker struct {
	mu    sync.Mutex
	reads map[string]time.Time
}

func newCustomFileTracker() *customFileTracker {
	return &customFileTracker{
		reads: make(map[string]time.Time),
	}
}

func (f *customFileTracker) RecordRead(_ context.Context, sessionID, path string) {
	f.mu.Lock()
	f.reads[sessionID+"\x00"+path] = time.Now()
	f.mu.Unlock()
}

func (f *customFileTracker) LastReadTime(_ context.Context, sessionID, path string) time.Time {
	f.mu.Lock()
	t := f.reads[sessionID+"\x00"+path]
	f.mu.Unlock()
	return t
}

func (f *customFileTracker) ListReadFiles(_ context.Context, sessionID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := sessionID + "\x00"
	var out []string
	for k := range f.reads {
		if strings.HasPrefix(k, prefix) {
			out = append(out, strings.TrimPrefix(k, prefix))
		}
	}
	return out, nil
}

type customHistoryService struct {
	*pubsub.Broker[history.File]
	mu    sync.Mutex
	files map[string][]history.File
	byID  map[string]history.File
}

func newCustomHistoryService() *customHistoryService {
	return &customHistoryService{
		Broker: pubsub.NewBroker[history.File](),
		files:  make(map[string][]history.File),
		byID:   make(map[string]history.File),
	}
}

func (h *customHistoryService) Create(_ context.Context, sessionID, path, content string) (history.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now().Unix()
	f := history.File{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Path:      path,
		Content:   content,
		Version:   history.InitialVersion,
		CreatedAt: now,
		UpdatedAt: now,
	}
	key := sessionID + "\x00" + path
	h.files[key] = append(h.files[key], f)
	h.byID[f.ID] = f
	return f, nil
}

func (h *customHistoryService) CreateVersion(_ context.Context, sessionID, path, content string) (history.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := sessionID + "\x00" + path
	versions := h.files[key]
	var ver int64
	if len(versions) > 0 {
		ver = versions[len(versions)-1].Version + 1
	}
	now := time.Now().Unix()
	f := history.File{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Path:      path,
		Content:   content,
		Version:   ver,
		CreatedAt: now,
		UpdatedAt: now,
	}
	h.files[key] = append(h.files[key], f)
	h.byID[f.ID] = f
	return f, nil
}

func (h *customHistoryService) Get(_ context.Context, id string) (history.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	f, ok := h.byID[id]
	if !ok {
		return history.File{}, sql.ErrNoRows
	}
	return f, nil
}

func (h *customHistoryService) GetByPathAndSession(_ context.Context, path, sessionID string) (history.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	versions := h.files[sessionID+"\x00"+path]
	if len(versions) == 0 {
		return history.File{}, sql.ErrNoRows
	}
	return versions[len(versions)-1], nil
}

func (h *customHistoryService) ListBySession(_ context.Context, sessionID string) ([]history.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	prefix := sessionID + "\x00"
	var out []history.File
	for k, list := range h.files {
		if strings.HasPrefix(k, prefix) {
			out = append(out, list...)
		}
	}
	return out, nil
}

func (h *customHistoryService) ListLatestSessionFiles(_ context.Context, sessionID string) ([]history.File, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	prefix := sessionID + "\x00"
	var out []history.File
	for k, list := range h.files {
		if strings.HasPrefix(k, prefix) && len(list) > 0 {
			out = append(out, list[len(list)-1])
		}
	}
	return out, nil
}

func (h *customHistoryService) Delete(_ context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.byID, id)
	return nil
}

func (h *customHistoryService) DeleteSessionFiles(_ context.Context, sessionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	prefix := sessionID + "\x00"
	for k, list := range h.files {
		if strings.HasPrefix(k, prefix) {
			for _, f := range list {
				delete(h.byID, f.ID)
			}
			delete(h.files, k)
		}
	}
	return nil
}
