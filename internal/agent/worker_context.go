package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/diff"
	"github.com/SpherePrime/CLI/internal/filepathext"
	"github.com/SpherePrime/CLI/internal/history"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	workerContextHeader       = "Background file context from team:"
	exploreContextBudgetBytes = 64 * 1024
	reviewContextBudgetBytes  = 96 * 1024
	maxFallbackReadLines      = 400
	maxModifiedContentBytes   = 24 * 1024
)

type lineRange struct {
	start int
	end   int
}

type modifiedFileContext struct {
	path        string
	baseline    string
	hasBaseline bool
}

func withContextNote(messages []fantasy.Message, note string) []fantasy.Message {
	if note == "" {
		return messages
	}
	insertAt := 0
	for index, entry := range messages {
		if entry.Role != fantasy.MessageRoleSystem {
			break
		}
		for _, part := range entry.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && strings.HasPrefix(text.Text, workerContextHeader) {
				return messages
			}
		}
		insertAt = index + 1
	}
	result := make([]fantasy.Message, 0, len(messages)+1)
	result = append(result, messages[:insertAt]...)
	result = append(result, fantasy.NewSystemMessage(note))
	return append(result, messages[insertAt:]...)
}

func (c *coordinator) applyContextSources(job *agentJob, sources []string) {
	if job == nil || len(sources) == 0 {
		return
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	seen := make(map[string]struct{}, len(job.contextFrom))
	for _, existing := range job.contextFrom {
		seen[existing] = struct{}{}
	}
	for _, raw := range sources {
		source := strings.TrimSpace(raw)
		if source == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		job.contextFrom = append(job.contextFrom, source)
	}
}

func (c *coordinator) snapshotWorkerWatermark(ctx context.Context, job *agentJob) {
	if c == nil || job == nil {
		return
	}
	job.mu.Lock()
	sessionID := job.sessionID
	job.mu.Unlock()
	if sessionID == "" {
		return
	}
	var msgs []message.Message
	if c.messages != nil {
		if listed, err := c.messages.List(ctx, sessionID); err == nil {
			msgs = listed
		}
	}
	var histEntries []history.File
	if c.history != nil {
		if listed, err := c.history.ListBySession(ctx, sessionID); err == nil {
			histEntries = listed
		}
	}
	job.mu.Lock()
	job.contextMsgWatermark = len(msgs)
	job.contextHistoryWatermark = len(histEntries)
	if job.contextMsgSeen == nil {
		job.contextMsgSeen = make(map[string]bool, len(msgs))
	}
	for _, m := range msgs {
		job.contextMsgSeen[m.ID] = true
	}
	if job.contextHistorySeen == nil {
		job.contextHistorySeen = make(map[string]bool, len(histEntries))
	}
	for _, entry := range histEntries {
		job.contextHistorySeen[entry.ID] = true
	}
	job.mu.Unlock()
}

func (c *coordinator) buildWorkerContextNote(ctx context.Context, job *agentJob, authorJob *agentJob) string {
	if c == nil || job == nil {
		return ""
	}
	switch job.agent {
	case config.AgentCode:
		return c.buildExploreToCodeContext(ctx, job)
	case config.AgentReview:
		if authorJob == nil {
			return ""
		}
		return c.buildCodeToReviewContext(ctx, authorJob)
	default:
		return ""
	}
}

func (c *coordinator) buildExploreToCodeContext(ctx context.Context, job *agentJob) string {
	job.mu.Lock()
	explicit := append([]string(nil), job.contextFrom...)
	job.contextFrom = nil
	rootSessionID := job.rootSessionID
	codeSessionID := job.sessionID
	consumed := maps.Clone(job.contextConsumed)
	job.mu.Unlock()

	sourceSessions := c.resolveExploreSources(ctx, rootSessionID, codeSessionID, explicit, consumed)
	if len(sourceSessions) == 0 {
		return ""
	}

	mergedRanges := map[string][]lineRange{}
	var orderedPaths []string
	seenPath := map[string]bool{}
	newCounts := map[string]int{}

	for _, srcSessionID := range sourceSessions {
		fileRanges, paths, msgCount := c.collectSessionReadFiles(ctx, srcSessionID)
		newCounts[srcSessionID] = msgCount
		for _, path := range paths {
			if !seenPath[path] {
				seenPath[path] = true
				orderedPaths = append(orderedPaths, path)
			}
			mergedRanges[path] = append(mergedRanges[path], fileRanges[path]...)
		}
	}

	job.mu.Lock()
	if job.contextConsumed == nil {
		job.contextConsumed = map[string]int{}
	}
	for sid, count := range newCounts {
		job.contextConsumed[sid] = count
	}
	job.mu.Unlock()

	if len(orderedPaths) == 0 {
		return ""
	}

	intro := workerContextHeader + "\nPreloaded read-only file contents from exploration (current on-disk state). Use these directly without re-reading unchanged ranges:\n\n"
	return c.formatReadSections(intro, orderedPaths, mergedRanges, exploreContextBudgetBytes)
}

func (c *coordinator) resolveExploreSources(
	ctx context.Context,
	rootSessionID string,
	codeSessionID string,
	explicit []string,
	consumed map[string]int,
) []string {
	lookupScope := rootSessionID
	if lookupScope == "" {
		lookupScope = codeSessionID
	}
	var sessions []string
	seen := map[string]bool{}
	addSession := func(sid string) {
		if sid == "" || sid == codeSessionID || seen[sid] {
			return
		}
		seen[sid] = true
		sessions = append(sessions, sid)
	}

	if len(explicit) > 0 {
		for _, target := range explicit {
			if worker, err := c.teamWorker(ctx, lookupScope, target); err == nil {
				addSession(worker.SessionID)
				continue
			}
			if found := c.findJob(target); found != nil {
				addSession(found.sessionID)
			}
		}
		return sessions
	}

	workers, err := c.AgentWorkers(ctx, lookupScope)
	if err != nil {
		return nil
	}

	var priorCodeUpdatedAt int64
	hasPriorCodeHistory := false
	if len(consumed) == 0 && c.messages != nil && codeSessionID != "" {
		if codeMsgs, msgErr := c.messages.List(ctx, codeSessionID); msgErr == nil && len(codeMsgs) > 0 {
			hasPriorCodeHistory = true
			for _, m := range codeMsgs {
				if m.CreatedAt > priorCodeUpdatedAt {
					priorCodeUpdatedAt = m.CreatedAt
				}
			}
		}
	}

	for _, w := range workers {
		if w.SessionID == "" || w.SessionID == codeSessionID {
			continue
		}
		if w.Agent != config.AgentExplore && w.Agent != config.AgentTask {
			continue
		}
		if w.Status != "done" && w.Status != "idle" {
			continue
		}
		if hasPriorCodeHistory && len(consumed) == 0 && w.UpdatedAt <= priorCodeUpdatedAt {
			continue
		}
		if prevCount, ok := consumed[w.SessionID]; ok {
			if c.messages == nil {
				continue
			}
			msgs, msgErr := c.messages.List(ctx, w.SessionID)
			if msgErr != nil || len(msgs) <= prevCount {
				continue
			}
		}
		addSession(w.SessionID)
	}
	return sessions
}

func (c *coordinator) buildCodeToReviewContext(ctx context.Context, authorJob *agentJob) string {
	authorJob.mu.Lock()
	authorSessionID := authorJob.sessionID
	msgWatermark := authorJob.contextMsgWatermark
	histWatermark := authorJob.contextHistoryWatermark
	msgSeen := maps.Clone(authorJob.contextMsgSeen)
	histSeen := maps.Clone(authorJob.contextHistorySeen)
	authorJob.mu.Unlock()

	if authorSessionID == "" {
		return ""
	}

	var msgs []message.Message
	if c.messages != nil {
		if listed, err := c.messages.List(ctx, authorSessionID); err == nil {
			msgs = listed
		}
	}
	var newMsgs []message.Message
	if msgSeen == nil && msgWatermark > 0 {
		msgStart := min(msgWatermark, len(msgs))
		newMsgs = msgs[msgStart:]
	} else {
		for _, m := range msgs {
			if !msgSeen[m.ID] {
				newMsgs = append(newMsgs, m)
			}
		}
	}

	var histEntries []history.File
	if c.history != nil {
		if listed, err := c.history.ListBySession(ctx, authorSessionID); err == nil {
			histEntries = listed
		}
	}
	var prevHist, newHist []history.File
	if histSeen == nil && histWatermark > 0 {
		histStart := min(histWatermark, len(histEntries))
		prevHist = histEntries[:histStart]
		newHist = histEntries[histStart:]
	} else {
		for _, entry := range histEntries {
			if histSeen[entry.ID] {
				prevHist = append(prevHist, entry)
			} else {
				newHist = append(newHist, entry)
			}
		}
	}

	prevLatestByPath := map[string]history.File{}
	for _, entry := range prevHist {
		resolved := c.resolveContextPath(entry.Path)
		if resolved == "" {
			continue
		}
		prevLatestByPath[resolved] = entry
	}

	modifiedMap := map[string]*modifiedFileContext{}
	var modifiedOrder []string
	ensureModified := func(path string) *modifiedFileContext {
		if existing, ok := modifiedMap[path]; ok {
			return existing
		}
		mod := &modifiedFileContext{path: path}
		if prev, ok := prevLatestByPath[path]; ok {
			mod.baseline = prev.Content
			mod.hasBaseline = true
		}
		modifiedMap[path] = mod
		modifiedOrder = append(modifiedOrder, path)
		return mod
	}

	for _, entry := range newHist {
		resolved := c.resolveContextPath(entry.Path)
		if resolved == "" {
			continue
		}
		mod := ensureModified(resolved)
		if !mod.hasBaseline {
			mod.baseline = entry.Content
			mod.hasBaseline = true
		}
	}

	errorCallIDs := collectErrorToolCallIDs(newMsgs)
	for _, msg := range newMsgs {
		for _, tc := range msg.ToolCalls() {
			if !modifiesFile(tc.Name) || errorCallIDs[tc.ID] {
				continue
			}
			rawPath := callFilePath(tc.Input)
			if rawPath == "" || strings.HasPrefix(rawPath, skills.BuiltinPrefix) {
				continue
			}
			resolved := c.resolveContextPath(rawPath)
			if resolved != "" {
				ensureModified(resolved)
			}
		}
	}

	includeFallback := msgWatermark == 0 && msgSeen == nil
	readRanges, readPaths := c.collectSessionReadFilesFromMsgs(ctx, authorSessionID, newMsgs, includeFallback)
	var readOnlyPaths []string
	for _, path := range readPaths {
		if _, isModified := modifiedMap[path]; isModified {
			continue
		}
		readOnlyPaths = append(readOnlyPaths, path)
	}

	authorJob.mu.Lock()
	if len(msgs) > authorJob.contextMsgWatermark {
		authorJob.contextMsgWatermark = len(msgs)
	}
	if len(histEntries) > authorJob.contextHistoryWatermark {
		authorJob.contextHistoryWatermark = len(histEntries)
	}
	if authorJob.contextMsgSeen == nil {
		authorJob.contextMsgSeen = make(map[string]bool, len(msgs))
	}
	for _, m := range msgs {
		authorJob.contextMsgSeen[m.ID] = true
	}
	if authorJob.contextHistorySeen == nil {
		authorJob.contextHistorySeen = make(map[string]bool, len(histEntries))
	}
	for _, entry := range histEntries {
		authorJob.contextHistorySeen[entry.ID] = true
	}
	authorJob.mu.Unlock()

	if len(modifiedOrder) == 0 && len(readOnlyPaths) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString(workerContextHeader)
	out.WriteString("\nPreloaded modified files, unified diffs from baseline, and read-only context from the author (current on-disk state). Use these directly instead of re-reading unchanged files/ranges:\n")

	budget := reviewContextBudgetBytes
	var omittedModified []string
	wroteAny := false

	for _, path := range modifiedOrder {
		mod := modifiedMap[path]
		section := c.formatModifiedSection(mod, budget-out.Len())
		if section == "" {
			omittedModified = append(omittedModified, c.displayContextPath(path))
			continue
		}
		if out.Len()+len(section) > budget {
			omittedModified = append(omittedModified, c.displayContextPath(path))
			continue
		}
		out.WriteString(section)
		wroteAny = true
	}

	var omittedReads []string
	if len(readOnlyPaths) > 0 {
		sectionHeader := "\n## Read-only context files\n"
		wroteReadHeader := false
		for _, path := range readOnlyPaths {
			ranges := mergeLineRanges(readRanges[path])
			block := c.formatFileRangesBlock(path, ranges)
			if block == "" {
				continue
			}
			needed := len(block)
			if !wroteReadHeader {
				needed += len(sectionHeader)
			}
			if out.Len()+needed > budget {
				omittedReads = append(omittedReads, c.displayContextPath(path))
				continue
			}
			if !wroteReadHeader {
				out.WriteString(sectionHeader)
				wroteReadHeader = true
			}
			out.WriteString(block)
			wroteAny = true
		}
	}

	if len(omittedModified) > 0 {
		out.WriteString("\nAdditional modified files omitted due to size budget:\n")
		for _, p := range omittedModified {
			fmt.Fprintf(&out, "- %s\n", p)
		}
	}
	if len(omittedReads) > 0 {
		out.WriteString("\nAdditional read files omitted due to size budget:\n")
		for _, p := range omittedReads {
			fmt.Fprintf(&out, "- %s\n", p)
		}
	}

	if !wroteAny && len(omittedModified) == 0 && len(omittedReads) == 0 {
		return ""
	}
	return out.String()
}

func (c *coordinator) formatModifiedSection(mod *modifiedFileContext, remainingBudget int) string {
	displayPath := c.displayContextPath(mod.path)
	rawBytes, err := os.ReadFile(mod.path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("\n### Modified: %s (deleted or missing on disk)\n", displayPath)
		}
		return ""
	}
	if !utf8.Valid(rawBytes) {
		return fmt.Sprintf("\n### Modified: %s (binary file, %d bytes)\n", displayPath, len(rawBytes))
	}
	currentContent := string(rawBytes)

	var b strings.Builder
	if mod.hasBaseline {
		unified, additions, removals := diff.GenerateDiff(mod.baseline, currentContent, displayPath)
		fmt.Fprintf(&b, "\n### Modified: %s (+%d -%d)\n", displayPath, additions, removals)
		if unified != "" {
			b.WriteString("```diff\n")
			b.WriteString(unified)
			if !strings.HasSuffix(unified, "\n") {
				b.WriteByte('\n')
			}
			b.WriteString("```\n")
		} else {
			b.WriteString("(no textual diff from baseline)\n")
		}
	} else {
		fmt.Fprintf(&b, "\n### Modified: %s\n", displayPath)
	}

	if remainingBudget <= b.Len()+128 {
		return b.String()
	}

	numbered := formatNumberedSlice(currentContent, 1, 0)
	if numbered != "" {
		contentBudget := min(maxModifiedContentBytes, remainingBudget-b.Len()-64)
		if len(numbered) <= contentBudget {
			b.WriteString("Current on-disk content:\n")
			b.WriteString(numbered)
			b.WriteByte('\n')
		} else if contentBudget > 256 {
			lines := strings.Split(numbered, "\n")
			var partial strings.Builder
			for _, line := range lines {
				if partial.Len()+len(line)+1 > contentBudget-64 {
					break
				}
				if partial.Len() > 0 {
					partial.WriteByte('\n')
				}
				partial.WriteString(line)
			}
			if partial.Len() > 0 {
				b.WriteString("Current on-disk content (truncated to budget):\n")
				b.WriteString(partial.String())
				b.WriteString("\n...\n")
			}
		}
	}
	return b.String()
}

func (c *coordinator) collectSessionReadFiles(ctx context.Context, sessionID string) (map[string][]lineRange, []string, int) {
	var msgs []message.Message
	if c.messages != nil {
		if listed, err := c.messages.List(ctx, sessionID); err == nil {
			msgs = listed
		}
	}
	fileRanges, orderedPaths := c.collectSessionReadFilesFromMsgs(ctx, sessionID, msgs, true)
	return fileRanges, orderedPaths, len(msgs)
}

func (c *coordinator) collectSessionReadFilesFromMsgs(ctx context.Context, sessionID string, msgs []message.Message, includeFallback bool) (map[string][]lineRange, []string) {
	fileRanges := map[string][]lineRange{}
	var orderedPaths []string
	seen := map[string]bool{}
	addRange := func(path string, rng lineRange) {
		if path == "" {
			return
		}
		if !seen[path] {
			seen[path] = true
			orderedPaths = append(orderedPaths, path)
		}
		fileRanges[path] = append(fileRanges[path], rng)
	}

	errorCallIDs := collectErrorToolCallIDs(msgs)
	for _, msg := range msgs {
		for _, tc := range msg.ToolCalls() {
			if tc.Name != tools.ViewToolName || errorCallIDs[tc.ID] {
				continue
			}
			var args tools.ViewParams
			if json.Unmarshal([]byte(tc.Input), &args) != nil {
				continue
			}
			rawPath := strings.TrimSpace(args.FilePath)
			if rawPath == "" || strings.HasPrefix(rawPath, skills.BuiltinPrefix) {
				continue
			}
			resolved := c.resolveContextPath(rawPath)
			if resolved == "" {
				continue
			}
			offset := max(args.Offset, 0)
			limit := args.Limit
			if limit <= 0 {
				limit = tools.DefaultReadLimit
			}
			startLine := offset + 1
			endLine := startLine + limit - 1
			addRange(resolved, lineRange{start: startLine, end: endLine})
		}
	}

	if includeFallback && c.filetracker != nil {
		if tracked, err := c.filetracker.ListReadFiles(ctx, sessionID); err == nil {
			for _, p := range tracked {
				if p == "" || strings.HasPrefix(p, skills.BuiltinPrefix) {
					continue
				}
				resolved := resolveTrackedFilePath(c.resolveContextPath(p))
				if resolved == "" || seen[resolved] {
					continue
				}
				addRange(resolved, lineRange{start: 1, end: maxFallbackReadLines})
			}
		}
	}

	return fileRanges, orderedPaths
}

func resolveTrackedFilePath(resolved string) string {
	if resolved == "" {
		return ""
	}
	if _, err := os.Stat(resolved); err == nil {
		return resolved
	}
	cwd, err := os.Getwd()
	if err != nil {
		return resolved
	}
	cleanCwd := filepath.Clean(cwd)
	if after, ok := strings.CutPrefix(resolved, cleanCwd); ok && after != "" {
		if _, statErr := os.Stat(after); statErr == nil {
			return after
		}
		trimmed := strings.TrimPrefix(after, string(filepath.Separator))
		if trimmed != "" {
			if _, statErr := os.Stat(trimmed); statErr == nil {
				return trimmed
			}
		}
	}
	return resolved
}

func collectErrorToolCallIDs(msgs []message.Message) map[string]bool {
	errorsByID := map[string]bool{}
	for _, msg := range msgs {
		for _, tr := range msg.ToolResults() {
			if tr.IsError && tr.ToolCallID != "" {
				errorsByID[tr.ToolCallID] = true
			}
		}
	}
	return errorsByID
}

func mergeLineRanges(ranges []lineRange) []lineRange {
	if len(ranges) <= 1 {
		return ranges
	}
	for _, r := range ranges {
		if r.end <= 0 {
			return []lineRange{{start: 1, end: 0}}
		}
	}
	sorted := append([]lineRange(nil), ranges...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start < sorted[j].start
		}
		return sorted[i].end < sorted[j].end
	})
	merged := []lineRange{sorted[0]}
	for _, next := range sorted[1:] {
		last := &merged[len(merged)-1]
		if next.start <= last.end+1 {
			if next.end > last.end {
				last.end = next.end
			}
		} else {
			merged = append(merged, next)
		}
	}
	return merged
}

func (c *coordinator) formatReadSections(intro string, orderedPaths []string, rangesByPath map[string][]lineRange, budget int) string {
	var out strings.Builder
	out.WriteString(intro)
	baseLen := out.Len()
	var omitted []string

	for _, path := range orderedPaths {
		merged := mergeLineRanges(rangesByPath[path])
		block := c.formatFileRangesBlock(path, merged)
		if block == "" {
			continue
		}
		if out.Len()+len(block) > budget {
			omitted = append(omitted, c.displayContextPath(path))
			continue
		}
		out.WriteString(block)
	}

	if out.Len() == baseLen && len(omitted) == 0 {
		return ""
	}
	if len(omitted) > 0 {
		out.WriteString("\nAdditional read files omitted due to size budget:\n")
		for _, p := range omitted {
			fmt.Fprintf(&out, "- %s\n", p)
		}
	}
	return out.String()
}

func (c *coordinator) formatFileRangesBlock(path string, ranges []lineRange) string {
	lines, ok := readFileLinesForContext(path)
	if !ok {
		return ""
	}
	displayPath := c.displayContextPath(path)
	var b strings.Builder
	for _, rng := range ranges {
		start := max(rng.start, 1)
		end := rng.end
		if end <= 0 || end > len(lines) {
			end = len(lines)
		}
		if len(lines) == 0 {
			fmt.Fprintf(&b, "### %s (empty file)\n", displayPath)
			continue
		}
		if start > len(lines) {
			continue
		}
		fmt.Fprintf(&b, "### %s (lines %d-%d)\n", displayPath, start, end)
		for i := start; i <= end; i++ {
			lineText := lines[i-1]
			if len(lineText) > tools.MaxLineLength {
				lineText = strings.ToValidUTF8(lineText[:tools.MaxLineLength], "") + "..."
			}
			b.WriteString(formatNumberedLine(i, lineText))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func readFileLinesForContext(path string) ([]string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > tools.MaxViewSize {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, tools.MaxViewSize+1))
	if err != nil || len(data) > tools.MaxViewSize || !utf8.Valid(data) {
		return nil, false
	}
	if len(data) == 0 {
		return nil, true
	}
	raw := string(data)
	raw = strings.TrimSuffix(raw, "\n")
	raw = strings.TrimSuffix(raw, "\r")
	lines := strings.Split(raw, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines, true
}

func formatNumberedSlice(content string, startLine int, maxLines int) string {
	if content == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(content, "\n")
	lines := strings.Split(trimmed, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	var b strings.Builder
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(formatNumberedLine(startLine+i, line))
	}
	return b.String()
}

func formatNumberedLine(lineNum int, line string) string {
	numStr := fmt.Sprintf("%d", lineNum)
	if len(numStr) >= 6 {
		return fmt.Sprintf("%s|%s", numStr, line)
	}
	return fmt.Sprintf("%6s|%s", numStr, line)
}

func (c *coordinator) resolveContextPath(rawPath string) string {
	p := strings.TrimSpace(rawPath)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if c != nil && c.cfg != nil && c.cfg.WorkingDir() != "" {
		return filepath.Clean(filepathext.SmartJoin(c.cfg.WorkingDir(), p))
	}
	return filepath.Clean(p)
}

func (c *coordinator) displayContextPath(path string) string {
	if c != nil && c.cfg != nil && c.cfg.WorkingDir() != "" {
		if rel, err := filepath.Rel(c.cfg.WorkingDir(), path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}
