package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	trivialMaxFiles            = 1
	trivialMaxChangedLines     = 10
	baseVerifyMaxAttempts      = 1
	enhancedVerifyMaxAttempts  = 3
	strictVerifyMaxAttempts    = 6
	strictVerifyStallThreshold = 3
)

type verifyTrackerContextKey struct{}

type runVerifyTracker struct {
	mu                      sync.Mutex
	seq                     int64
	mutatedFiles            map[string]struct{}
	createdFiles            map[string]struct{}
	changedLines            int
	lastMutationSeq         int64
	lastLocalCheckSeq       int64
	lastApprovedReviewSeq   int64
	lastReviewDefectSig     string
	consecutiveRepeatReview int
	noProgressGateRounds    int
	lastGateSeqSnapshot     int64
}

func newRunVerifyTracker() *runVerifyTracker {
	return &runVerifyTracker{
		mutatedFiles: make(map[string]struct{}),
		createdFiles: make(map[string]struct{}),
	}
}

func withVerifyTracker(ctx context.Context, tracker *runVerifyTracker) context.Context {
	if tracker == nil {
		return ctx
	}
	return context.WithValue(ctx, verifyTrackerContextKey{}, tracker)
}

func verifyTrackerFromContext(ctx context.Context) *runVerifyTracker {
	if ctx == nil {
		return nil
	}
	tracker, _ := ctx.Value(verifyTrackerContextKey{}).(*runVerifyTracker)
	return tracker
}

func (t *runVerifyTracker) recordMutation(filePath string, lines int, created bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	t.lastMutationSeq = t.seq
	if path := strings.TrimSpace(filePath); path != "" {
		t.mutatedFiles[path] = struct{}{}
		if created {
			t.createdFiles[path] = struct{}{}
		}
	}
	if lines < 1 {
		lines = 1
	}
	t.changedLines += lines
}

func (t *runVerifyTracker) recordLocalCheck() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	t.lastLocalCheckSeq = t.seq
}

func (t *runVerifyTracker) recordReviewResult(output string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	if reviewApproved(output) {
		t.lastApprovedReviewSeq = t.seq
		t.lastReviewDefectSig = ""
		t.consecutiveRepeatReview = 0
		return
	}
	sig := reviewDefectSignature(output)
	if sig != "" && sig == t.lastReviewDefectSig {
		t.consecutiveRepeatReview++
	} else {
		t.lastReviewDefectSig = sig
		t.consecutiveRepeatReview = 1
	}
}

func (t *runVerifyTracker) hasMutations() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastMutationSeq > 0
}

func (t *runVerifyTracker) isTrivialMutation() bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastMutationSeq == 0 {
		return true
	}
	return len(t.createdFiles) == 0 && len(t.mutatedFiles) <= trivialMaxFiles && t.changedLines <= trivialMaxChangedLines
}

func (t *runVerifyTracker) hasFreshLocalCheck() bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastMutationSeq == 0 || t.lastLocalCheckSeq > t.lastMutationSeq || t.lastApprovedReviewSeq > t.lastMutationSeq
}

func (t *runVerifyTracker) hasFreshApprovedReview() bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastMutationSeq == 0 || t.lastApprovedReviewSeq > t.lastMutationSeq
}

func (t *runVerifyTracker) reviewStalled() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.consecutiveRepeatReview >= strictVerifyStallThreshold
}

func (t *runVerifyTracker) nextGatePrompt(strictVerify bool, reasoningMode string, canDelegate bool, attempt int) (string, bool) {
	if t == nil || !t.hasMutations() {
		return "", false
	}

	t.mu.Lock()
	currentSeq := t.seq
	if attempt > 0 && currentSeq == t.lastGateSeqSnapshot {
		t.noProgressGateRounds++
	} else {
		t.noProgressGateRounds = 0
	}
	t.lastGateSeqSnapshot = currentSeq
	noProgress := t.noProgressGateRounds >= 2
	fileCount := len(t.mutatedFiles)
	lineCount := t.changedLines
	t.mu.Unlock()

	if noProgress {
		return "", false
	}

	if strictVerify {
		if t.isTrivialMutation() {
			if t.hasFreshLocalCheck() || attempt >= 2 {
				return "", false
			}
			return "Strict Verify (trivial edit): run a fresh local verification check (build, tests, diagnostics, or live interactive check for UI logic) after your last edit before finishing.", true
		}
		if t.hasFreshApprovedReview() || t.reviewStalled() || attempt >= strictVerifyMaxAttempts {
			return "", false
		}
		if canDelegate {
			return fmt.Sprintf("Strict Verify gate: %d file(s) (~%d lines) were changed and there is no fresh VERDICT: APPROVED from an independent review agent after your last edit. Launch a review subagent (agent=\"review\") with the requirements, changed files, and target surface, fix every defect it reports, and repeat until VERDICT: APPROVED is returned after the final edit.", fileCount, lineCount), true
		}
		if t.hasFreshLocalCheck() || attempt >= enhancedVerifyMaxAttempts {
			return "", false
		}
		return "Strict Verify gate: files were edited after your last clean verification pass. Run fresh checks appropriate for the target surface (build, lint, tests, live execution, or interactive UI clicks/console checks), fix any issues found, and verify again after the final edit.", true
	}

	if isEnhancedVerificationReasoning(reasoningMode) {
		if t.hasFreshLocalCheck() || attempt >= enhancedVerifyMaxAttempts {
			return "", false
		}
		return "Verification gate: files were modified after your last clean check. Run a fresh verification pass matched to the target surface (build, lint, tests, live execution, or interactive browser/app clicks for UI logic, plus targeted screenshots only if visuals changed), fix any issues found, and confirm clean results after the final edit.", true
	}

	if t.hasFreshLocalCheck() || attempt >= baseVerifyMaxAttempts {
		return "", false
	}
	return "Files were modified after your last verification check. Run a quick relevant check (build, tests, diagnostics, or live click check for UI logic) after the last edit, then finish your response.", true
}

func reviewDefectSignature(report string) string {
	idx := strings.Index(strings.ToUpper(report), "DEFECTS:")
	if idx < 0 {
		return ""
	}
	rest := report[idx+len("DEFECTS:"):]
	for _, stop := range []string{"\nSCREENSHOTS:", "\nCHECKS:", "\nCRITERIA:"} {
		if end := strings.Index(strings.ToUpper(rest), stop); end >= 0 {
			rest = rest[:end]
		}
	}
	var lines []string
	for _, line := range strings.Split(rest, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			lines = append(lines, strings.ToLower(trimmed))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "|")
}

type verifyTrackerTool struct {
	inner fantasy.AgentTool
}

func wrapToolsWithVerifyTracker(tools []fantasy.AgentTool) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = &verifyTrackerTool{inner: tool}
	}
	return out
}

func (v *verifyTrackerTool) Info() fantasy.ToolInfo { return v.inner.Info() }

func (v *verifyTrackerTool) ProviderOptions() fantasy.ProviderOptions {
	return v.inner.ProviderOptions()
}

func (v *verifyTrackerTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	v.inner.SetProviderOptions(opts)
}

func (v *verifyTrackerTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	resp, err := v.inner.Run(ctx, call)
	tracker := verifyTrackerFromContext(ctx)
	if tracker == nil || err != nil || resp.IsError {
		return resp, err
	}
	recordToolOutcomeForVerify(tracker, call.Name, call.Input)
	return resp, err
}

func recordToolOutcomeForVerify(tracker *runVerifyTracker, toolName, input string) {
	switch toolName {
	case "edit":
		var args struct {
			FilePath  string `json:"file_path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		_ = json.Unmarshal([]byte(input), &args)
		tracker.recordMutation(args.FilePath, estimateChangedLines(args.OldString, args.NewString), false)
	case "multiedit":
		var args struct {
			FilePath string `json:"file_path"`
			Edits    []struct {
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			} `json:"edits"`
		}
		_ = json.Unmarshal([]byte(input), &args)
		lines := 0
		for _, e := range args.Edits {
			lines += estimateChangedLines(e.OldString, e.NewString)
		}
		tracker.recordMutation(args.FilePath, lines, false)
	case "write":
		var args struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		_ = json.Unmarshal([]byte(input), &args)
		lines := countTextLines(args.Content)
		tracker.recordMutation(args.FilePath, lines, true)
	case "lsp_rename", "lsp_replace_symbol":
		var args struct {
			FilePath    string `json:"file_path"`
			Path        string `json:"path"`
			Replacement string `json:"replacement"`
		}
		_ = json.Unmarshal([]byte(input), &args)
		path := args.FilePath
		if path == "" {
			path = args.Path
		}
		lines := countTextLines(args.Replacement)
		if lines < 2 {
			lines = 2
		}
		tracker.recordMutation(path, lines, false)
	case "bash", "lsp_diagnostics":
		tracker.recordLocalCheck()
	default:
		if isVerificationMCPTool(toolName) {
			tracker.recordLocalCheck()
		}
	}
}

func isVerificationMCPTool(name string) bool {
	if strings.Contains(name, "browser_run") ||
		strings.Contains(name, "browser_click") ||
		strings.Contains(name, "browser_type") ||
		strings.Contains(name, "browser_evaluate") ||
		strings.Contains(name, "browser_screenshot") ||
		strings.Contains(name, "browser_snapshot") ||
		strings.Contains(name, "browser_press_key") {
		return true
	}
	if strings.Contains(name, "app_audit") ||
		strings.Contains(name, "app_check") ||
		strings.Contains(name, "app_test_run") ||
		strings.Contains(name, "app_test_explore") ||
		strings.Contains(name, "app_click") ||
		strings.Contains(name, "app_type") ||
		strings.Contains(name, "app_snapshot") ||
		strings.Contains(name, "app_screenshot") ||
		strings.Contains(name, "window_view") {
		return true
	}
	return false
}

func estimateChangedLines(oldStr, newStr string) int {
	oldLines := countTextLines(oldStr)
	newLines := countTextLines(newStr)
	if oldLines > newLines {
		return oldLines
	}
	return newLines
}

func countTextLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
