package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type captureStepsModel struct {
	fastModel
	mu           sync.Mutex
	stepMessages [][]fantasy.Message
}

func (m *captureStepsModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	step := len(m.stepMessages)
	copied := make([]fantasy.Message, len(call.Prompt))
	copy(copied, call.Prompt)
	m.stepMessages = append(m.stepMessages, copied)
	m.mu.Unlock()

	return func(yield func(fantasy.StreamPart) bool) {
		if step == 0 {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "tc-1", ToolCallName: "ping", ToolCallInput: `{}`})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t-1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t-1", Delta: "done"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t-1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func TestWithContextNote_NoDuplicationAndOrderedAfterSystem(t *testing.T) {
	t.Parallel()

	note := workerContextHeader + "\nfile a"
	msgs := []fantasy.Message{
		fantasy.NewSystemMessage("base system"),
		fantasy.NewUserMessage("user prompt"),
	}

	injected := withContextNote(msgs, note)
	require.Len(t, injected, 3)
	require.Equal(t, fantasy.MessageRoleSystem, injected[0].Role)
	require.Equal(t, fantasy.MessageRoleSystem, injected[1].Role)
	require.Equal(t, fantasy.MessageRoleUser, injected[2].Role)
	part, ok := fantasy.AsMessagePart[fantasy.TextPart](injected[1].Content[0])
	require.True(t, ok)
	require.Equal(t, note, part.Text)

	injectedAgain := withContextNote(injected, note)
	require.Len(t, injectedAgain, 3)
	require.Len(t, withContextNote(msgs, ""), 2)
}

func TestWithContextNote_TransientAcrossStepsNotSavedInDB(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "Transient Context")
	require.NoError(t, err)

	model := &captureStepsModel{}
	pingTool := fantasy.NewAgentTool("ping", "ping", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("pong"), nil
	})
	sa := testSessionAgent(env, model, fastModel{}, "base system", pingTool)

	note := workerContextHeader + "\n### main.go (lines 1-2)\n     1|package main\n"
	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID:   sess.ID,
		Prompt:      "implement change",
		ContextNote: note,
	})
	require.NoError(t, err)

	model.mu.Lock()
	steps := model.stepMessages
	model.mu.Unlock()
	require.Len(t, steps, 2)

	for stepIdx, msgs := range steps {
		contextMatches := 0
		for _, m := range msgs {
			if m.Role != fantasy.MessageRoleSystem {
				continue
			}
			for _, p := range m.Content {
				if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](p); ok && strings.HasPrefix(tp.Text, workerContextHeader) {
					contextMatches++
				}
			}
		}
		require.Equal(t, 1, contextMatches, "step %d should carry context note exactly once", stepIdx)
	}

	persisted, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	for _, m := range persisted {
		require.NotEqual(t, message.System, m.Role)
		require.NotContains(t, m.Content().Text, workerContextHeader)
	}
}

func recordViewCall(t *testing.T, env fakeEnv, sessionID, callID, filePath string, offset, limit int) {
	t.Helper()
	input, err := json.Marshal(tools.ViewParams{
		FilePath: filePath,
		Offset:   offset,
		Limit:    limit,
	})
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       callID,
				Name:     tools.ViewToolName,
				Input:    string(input),
				Finished: true,
			},
		},
	})
	require.NoError(t, err)
}

func TestBuildExploreToCodeContext_MergesDedupsAndTruncates(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.history = env.history
	c.filetracker = *env.filetracker

	root, err := env.sessions.Create(t.Context(), "root")
	require.NoError(t, err)

	targetFile := filepath.Join(env.workingDir, "app.go")
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d original", i))
	}
	require.NoError(t, os.WriteFile(targetFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	fallbackDir := t.TempDir()
	fallbackFile := filepath.Join(fallbackDir, "tracked.go")
	require.NoError(t, os.WriteFile(fallbackFile, []byte("package tracked\nconst Version = 2\n"), 0o644))

	exploreSess1, err := env.sessions.CreateTaskSession(t.Context(), "exp-1", root.ID, "Agent Session: explore | scout-a")
	require.NoError(t, err)
	recordViewCall(t, env, exploreSess1.ID, "v1", targetFile, 0, 5)
	recordViewCall(t, env, exploreSess1.ID, "v2", targetFile, 3, 5)
	recordViewCall(t, env, exploreSess1.ID, "v3", "prime://skills/prime-designer/SKILL.md", 0, 0)

	exploreSess2, err := env.sessions.CreateTaskSession(t.Context(), "exp-2", root.ID, "Agent Session: explore | scout-b")
	require.NoError(t, err)
	recordViewCall(t, env, exploreSess2.ID, "v4", targetFile, 8, 4)
	(*env.filetracker).RecordRead(t.Context(), exploreSess2.ID, fallbackFile)

	// Update disk content after explore ran to verify current on-disk state is served.
	lines[1] = "line 2 updated-on-disk"
	require.NoError(t, os.WriteFile(targetFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	runningExplore, err := env.sessions.CreateTaskSession(t.Context(), "exp-running", root.ID, "Agent Session: explore | busy")
	require.NoError(t, err)
	runningFile := filepath.Join(env.workingDir, "busy.go")
	require.NoError(t, os.WriteFile(runningFile, []byte("package busy\n"), 0o644))
	recordViewCall(t, env, runningExplore.ID, "v5", runningFile, 0, 5)

	c.jobMu.Lock()
	c.agentJobs["exp-1"] = &agentJob{
		id: "exp-1", agent: config.AgentExplore, sessionID: exploreSess1.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "scout-a", status: "done",
	}
	c.agentJobs["exp-2"] = &agentJob{
		id: "exp-2", agent: config.AgentExplore, sessionID: exploreSess2.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "scout-b", status: "done",
	}
	c.agentJobs["exp-running"] = &agentJob{
		id: "exp-running", agent: config.AgentExplore, sessionID: runningExplore.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "busy", status: "running",
	}
	c.jobMu.Unlock()

	codeSess, err := env.sessions.CreateTaskSession(t.Context(), "code-1", root.ID, "Agent Session: code | impl")
	require.NoError(t, err)
	codeJob := &agentJob{
		id: "code-1", agent: config.AgentCode, sessionID: codeSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "impl",
		prompt: "Update app.go and tracked.go",
	}

	note := c.buildWorkerContextNote(t.Context(), codeJob, nil, codeJob.prompt)
	require.Contains(t, note, workerContextHeader)
	require.Contains(t, note, "app.go (lines 1-12)")
	require.Contains(t, note, "line 2 updated-on-disk")
	require.Contains(t, note, "tracked.go (lines 1-2)")
	require.Contains(t, note, "const Version = 2")
	require.NotContains(t, note, "prime://")
	require.NotContains(t, note, "busy.go")

	// Second build without new explore sources or context_from produces nothing.
	require.Empty(t, c.buildWorkerContextNote(t.Context(), codeJob, nil, ""))

	// Explicit context_from targets a specific worker even on follow-up.
	c.applyContextSources(codeJob, []string{"scout-a"})
	c.applyContextFiles(codeJob, []string{"app.go"})
	explicitNote := c.buildWorkerContextNote(t.Context(), codeJob, nil, "follow up")
	require.Contains(t, explicitNote, "app.go (lines 1-8)")
	require.NotContains(t, explicitNote, "tracked.go")
}

func TestBuildExploreToCodeContext_EnforcesBudget(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)

	root, err := env.sessions.Create(t.Context(), "root")
	require.NoError(t, err)
	expSess, err := env.sessions.CreateTaskSession(t.Context(), "exp-big", root.ID, "Agent Session: explore | big")
	require.NoError(t, err)

	var paths []string
	bigLine := strings.Repeat("x", 1500)
	var chunk []string
	for range 30 {
		chunk = append(chunk, bigLine)
	}
	body := strings.Join(chunk, "\n") + "\n"
	for i := range 4 {
		p := filepath.Join(env.workingDir, fmt.Sprintf("large_%d.txt", i))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		paths = append(paths, p)
		recordViewCall(t, env, expSess.ID, fmt.Sprintf("vc-%d", i), p, 0, 50)
	}

	c.jobMu.Lock()
	c.agentJobs["exp-big"] = &agentJob{
		id: "exp-big", agent: config.AgentExplore, sessionID: expSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "big", status: "done",
	}
	c.jobMu.Unlock()

	codeJob := &agentJob{
		id: "code-big", agent: config.AgentCode, sessionID: "code-big-s", rootSessionID: root.ID,
		contextFiles: []string{"large_0.txt", "large_1.txt", "large_2.txt", "large_3.txt"},
	}
	note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "")
	require.NotEmpty(t, note)
	require.LessOrEqual(t, len(note), exploreContextBudgetBytes)
	require.Contains(t, note, "Additional read files omitted due to size budget:")
	require.Contains(t, note, filepath.Base(paths[3]))
}

func TestBuildCodeToReviewContext_DiffAndWatermarkDelta(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.history = env.history
	c.filetracker = *env.filetracker

	root, err := env.sessions.Create(t.Context(), "root")
	require.NoError(t, err)
	codeSess, err := env.sessions.CreateTaskSession(t.Context(), "code-1", root.ID, "Agent Session: code | feat")
	require.NoError(t, err)
	reviewSess, err := env.sessions.CreateTaskSession(t.Context(), "rev-1", root.ID, "Agent Session: review | rev")
	require.NoError(t, err)

	authorJob := &agentJob{
		id: "code-1", agent: config.AgentCode, sessionID: codeSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "feat",
	}
	reviewJob := &agentJob{
		id: "rev-1", agent: config.AgentReview, sessionID: reviewSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "rev", author: authorJob.id,
	}

	editedPath := filepath.Join(env.workingDir, "handler.go")
	createdPath := filepath.Join(env.workingDir, "helper.go")
	readOnlyPath := filepath.Join(env.workingDir, "config.go")
	laterReadPath := filepath.Join(env.workingDir, "types.go")

	v0 := "package app\nfunc Handle() string { return \"old\" }\n"
	v1 := "package app\nfunc Handle() string { return \"round1\" }\n"
	createdV1 := "package app\nfunc Help() int { return 1 }\n"
	require.NoError(t, os.WriteFile(editedPath, []byte(v1), 0o644))
	require.NoError(t, os.WriteFile(createdPath, []byte(createdV1), 0o644))
	require.NoError(t, os.WriteFile(readOnlyPath, []byte("package app\nconst Port = 8080\n"), 0o644))
	require.NoError(t, os.WriteFile(laterReadPath, []byte("package app\ntype User struct{ ID string }\n"), 0o644))

	_, err = env.history.Create(t.Context(), codeSess.ID, editedPath, v0)
	require.NoError(t, err)
	_, err = env.history.CreateVersion(t.Context(), codeSess.ID, editedPath, v1)
	require.NoError(t, err)
	_, err = env.history.Create(t.Context(), codeSess.ID, createdPath, "")
	require.NoError(t, err)
	_, err = env.history.CreateVersion(t.Context(), codeSess.ID, createdPath, createdV1)
	require.NoError(t, err)

	recordViewCall(t, env, codeSess.ID, "r1", readOnlyPath, 0, 50)
	recordViewCall(t, env, codeSess.ID, "r2", editedPath, 0, 50)

	round1Note := c.buildWorkerContextNote(t.Context(), reviewJob, authorJob, "")
	require.Contains(t, round1Note, workerContextHeader)
	require.Contains(t, round1Note, "### Modified: handler.go")
	require.Contains(t, round1Note, `-func Handle() string { return "old" }`)
	require.Contains(t, round1Note, `+func Handle() string { return "round1" }`)
	require.Contains(t, round1Note, "### Modified: helper.go")
	require.Contains(t, round1Note, `+func Help() int { return 1 }`)
	require.Contains(t, round1Note, "## Read-only context files")
	require.Contains(t, round1Note, "config.go (lines 1-2)")
	require.NotContains(t, round1Note, "handler.go (lines")

	// Round 2: author edits handler.go again and reads types.go.
	v2 := "package app\nfunc Handle() string { return \"round2\" }\n"
	require.NoError(t, os.WriteFile(editedPath, []byte(v2), 0o644))
	_, err = env.history.CreateVersion(t.Context(), codeSess.ID, editedPath, v2)
	require.NoError(t, err)
	recordViewCall(t, env, codeSess.ID, "r3", laterReadPath, 0, 50)

	round2Note := c.buildWorkerContextNote(t.Context(), reviewJob, authorJob, "")
	require.Contains(t, round2Note, "### Modified: handler.go")
	require.Contains(t, round2Note, `-func Handle() string { return "round1" }`)
	require.Contains(t, round2Note, `+func Handle() string { return "round2" }`)
	require.NotContains(t, round2Note, "helper.go")
	require.NotContains(t, round2Note, "config.go")
	require.Contains(t, round2Note, "types.go (lines 1-2)")

	// Follow-up watermark snapshot suppresses prior edits until a follow-up edit occurs.
	c.snapshotWorkerWatermark(t.Context(), authorJob)
	require.Empty(t, c.buildWorkerContextNote(t.Context(), reviewJob, authorJob, ""))
}

func TestBuildCodeToReviewContext_NewLowerVersionFileInRound2(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.history = env.history
	c.filetracker = *env.filetracker

	root, err := env.sessions.Create(t.Context(), "root")
	require.NoError(t, err)
	codeSess, err := env.sessions.CreateTaskSession(t.Context(), "code-mf", root.ID, "Agent Session: code | mf")
	require.NoError(t, err)
	reviewSess, err := env.sessions.CreateTaskSession(t.Context(), "rev-mf", root.ID, "Agent Session: review | mf-rev")
	require.NoError(t, err)

	authorJob := &agentJob{
		id: "code-mf", agent: config.AgentCode, sessionID: codeSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "mf",
	}
	reviewJob := &agentJob{
		id: "rev-mf", agent: config.AgentReview, sessionID: reviewSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "mf-rev", author: authorJob.id,
	}

	fileA := filepath.Join(env.workingDir, "a.go")
	require.NoError(t, os.WriteFile(fileA, []byte("package main\nfunc A() int { return 2 }\n"), 0o644))
	_, err = env.history.Create(t.Context(), codeSess.ID, fileA, "package main\nfunc A() int { return 0 }\n")
	require.NoError(t, err)
	_, err = env.history.CreateVersion(t.Context(), codeSess.ID, fileA, "package main\nfunc A() int { return 1 }\n")
	require.NoError(t, err)
	_, err = env.history.CreateVersion(t.Context(), codeSess.ID, fileA, "package main\nfunc A() int { return 2 }\n")
	require.NoError(t, err)

	r1 := c.buildWorkerContextNote(t.Context(), reviewJob, authorJob, "")
	require.Contains(t, r1, "### Modified: a.go")
	require.Contains(t, r1, `-func A() int { return 0 }`)
	require.Contains(t, r1, `+func A() int { return 2 }`)

	// Round 2 creates a brand-new file (versions 0 and 1, lower than a.go's version 2) without touching a.go.
	fileB := filepath.Join(env.workingDir, "b.go")
	fileBContent := "package main\nfunc B() string { return \"new\" }\n"
	require.NoError(t, os.WriteFile(fileB, []byte(fileBContent), 0o644))
	_, err = env.history.Create(t.Context(), codeSess.ID, fileB, "")
	require.NoError(t, err)
	_, err = env.history.CreateVersion(t.Context(), codeSess.ID, fileB, fileBContent)
	require.NoError(t, err)

	r2 := c.buildWorkerContextNote(t.Context(), reviewJob, authorJob, "")
	require.Contains(t, r2, "### Modified: b.go")
	require.Contains(t, r2, `+func B() string { return "new" }`)
	require.NotContains(t, r2, "a.go")
}

func TestExecuteWorker_WiresContextNoteForCodeAndReview(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.history = env.history
	c.filetracker = *env.filetracker

	root, err := env.sessions.Create(t.Context(), "root")
	require.NoError(t, err)

	sharedFile := filepath.Join(env.workingDir, "service.go")
	require.NoError(t, os.WriteFile(sharedFile, []byte("package svc\nfunc Run() {}\n"), 0o644))

	expSess, err := env.sessions.CreateTaskSession(t.Context(), "exp-live", root.ID, "Agent Session: explore | exp")
	require.NoError(t, err)
	recordViewCall(t, env, expSess.ID, "ev1", sharedFile, 0, 20)
	c.jobMu.Lock()
	c.agentJobs["exp-live"] = &agentJob{
		id: "exp-live", agent: config.AgentExplore, sessionID: expSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "exp", status: "done",
	}
	c.jobMu.Unlock()

	codeCalls := make(chan SessionAgentCall, 1)
	codeAgent := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		codeCalls <- call
		edited := "package svc\nfunc Run() { println(\"ok\") }\n"
		require.NoError(t, os.WriteFile(sharedFile, []byte(edited), 0o644))
		_, herr := env.history.Create(ctx, call.SessionID, sharedFile, "package svc\nfunc Run() {}\n")
		require.NoError(t, herr)
		_, herr = env.history.CreateVersion(ctx, call.SessionID, sharedFile, edited)
		require.NoError(t, herr)
		return agentResultWithText("implemented"), nil
	})

	reviewCalls := make(chan SessionAgentCall, 1)
	reviewAgent := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		reviewCalls <- call
		return agentResultWithText("VERDICT: APPROVED\nROUND: 1\nSCORE: design n/a, functionality 10/10, code 10/10\nLIKED: yes, clean\nCRITERIA:\n- 1. ok - PASS - done\nDEFECTS: none\nCHECKS: unit"), nil
	})

	revJob, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          reviewAgent,
		SessionID:      root.ID,
		AgentMessageID: "m1",
		ToolCallID:     "rev-call",
		Prompt:         "review it",
		SessionTitle:   "Agent Session: review | rev",
		Dormant:        true,
		AwaitHandoff:   true,
	}, config.AgentReview)
	require.NoError(t, err)

	codeJob, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          codeAgent,
		SessionID:      root.ID,
		AgentMessageID: "m1",
		ToolCallID:     "code-call",
		Prompt:         "update Run in service.go",
		SessionTitle:   "Agent Session: code | impl",
		Reviewer:       "rev",
	}, config.AgentCode)
	require.NoError(t, err)

	var codeCall, reviewCall SessionAgentCall
	select {
	case codeCall = <-codeCalls:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for code worker call")
	}
	require.Contains(t, codeCall.ContextNote, workerContextHeader)
	require.Contains(t, codeCall.ContextNote, "service.go (lines 1-2)")

	select {
	case reviewCall = <-reviewCalls:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for review worker call")
	}
	require.Contains(t, reviewCall.ContextNote, workerContextHeader)
	require.Contains(t, reviewCall.ContextNote, "### Modified: service.go")
	require.Contains(t, reviewCall.ContextNote, `+func Run() { println("ok") }`)
	require.True(t, codeJob.wait(t.Context(), 3*time.Second))
	require.True(t, revJob.wait(t.Context(), 3*time.Second))
}

func setupExploreReadsFixture(t *testing.T, relPaths []string) (fakeEnv, *coordinator, string, string) {
	t.Helper()
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.history = env.history
	c.filetracker = *env.filetracker

	root, err := env.sessions.Create(t.Context(), "root")
	require.NoError(t, err)
	expSess, err := env.sessions.CreateTaskSession(t.Context(), "exp-filter", root.ID, "Agent Session: explore | exp")
	require.NoError(t, err)

	for i, rel := range relPaths {
		abs := filepath.Join(env.workingDir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, fmt.Appendf(nil, "content of %s\n", rel), 0o644))
		recordViewCall(t, env, expSess.ID, fmt.Sprintf("view-%d", i), abs, 0, 20)
	}

	c.jobMu.Lock()
	c.agentJobs["exp-filter"] = &agentJob{
		id: "exp-filter", agent: config.AgentExplore, sessionID: expSess.ID,
		parentSessionID: root.ID, rootSessionID: root.ID, name: "exp", status: "done",
	}
	c.jobMu.Unlock()

	return env, c, root.ID, expSess.ID
}

func TestBuildExploreToCodeContext_FiltersOnlyTargetedFiles(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go", "pkg/one.go", "pkg/two.go", "other/three.go"}

	t.Run("explicit context_files excludes unrelated files even if named in prompt", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-1", agent: config.AgentCode, sessionID: "code-s-1",
			rootSessionID: rootID, contextFiles: []string{"a.go"},
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update a.go to match b.go and c.go")
		require.Contains(t, note, "### a.go (lines 1-1)")
		require.Contains(t, note, "content of a.go")
		require.NotContains(t, note, "b.go")
		require.NotContains(t, note, "c.go")
		require.NotContains(t, note, "pkg/one.go")
		require.NotContains(t, note, "other/three.go")
	})

	t.Run("prompt filename with backticks and line suffix", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-2", agent: config.AgentCode, sessionID: "code-s-2",
			rootSessionID: rootID,
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Fix issue in `a.go:42` and clean up logic.")
		require.Contains(t, note, "### a.go (lines 1-1)")
		require.NotContains(t, note, "b.go")
		require.NotContains(t, note, "c.go")
	})

	t.Run("directory prefix in context_files", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-3", agent: config.AgentCode, sessionID: "code-s-3",
			rootSessionID: rootID, contextFiles: []string{"pkg/"},
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update pkg handlers")
		require.Contains(t, note, "pkg/one.go")
		require.Contains(t, note, "pkg/two.go")
		require.NotContains(t, note, "a.go")
		require.NotContains(t, note, "b.go")
		require.NotContains(t, note, "other/three.go")
	})

	t.Run("directory segment without trailing slash in context_files", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-4", agent: config.AgentCode, sessionID: "code-s-4",
			rootSessionID: rootID, contextFiles: []string{"pkg"},
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update pkg handlers")
		require.Contains(t, note, "pkg/one.go")
		require.Contains(t, note, "pkg/two.go")
		require.NotContains(t, note, "other/three.go")
	})
}

func TestBuildExploreToCodeContext_AmbiguousBasenameVsSpecificSuffixPath(t *testing.T) {
	files := []string{"desktop/src/App.tsx", "mobile/src/App.tsx", "desktop/src/Other.tsx"}

	t.Run("specific path in prompt does not match sibling basename", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-1", agent: config.AgentCode, sessionID: "code-s-1",
			rootSessionID: rootID,
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update `desktop/src/App.tsx#L10` layout")
		require.Contains(t, note, "desktop/src/App.tsx")
		require.NotContains(t, note, "mobile/src/App.tsx")
		require.NotContains(t, note, "Other.tsx")
	})

	t.Run("markdown link with basename and specific path keeps only specific path", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-2", agent: config.AgentCode, sessionID: "code-s-2",
			rootSessionID: rootID,
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update [App.tsx](desktop/src/App.tsx)")
		require.Contains(t, note, "desktop/src/App.tsx")
		require.NotContains(t, note, "mobile/src/App.tsx")
	})

	t.Run("bare basename in prompt matches matching basename files", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-3", agent: config.AgentCode, sessionID: "code-s-3",
			rootSessionID: rootID,
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update App.tsx")
		require.Contains(t, note, "desktop/src/App.tsx")
		require.Contains(t, note, "mobile/src/App.tsx")
		require.NotContains(t, note, "Other.tsx")
	})

	t.Run("suffix path in context_files matches suffix", func(t *testing.T) {
		_, c, rootID, _ := setupExploreReadsFixture(t, files)
		codeJob := &agentJob{
			id: "code-4", agent: config.AgentCode, sessionID: "code-s-4",
			rootSessionID: rootID, contextFiles: []string{"src/App.tsx"},
		}
		note := c.buildWorkerContextNote(t.Context(), codeJob, nil, "Update component")
		require.Contains(t, note, "desktop/src/App.tsx")
		require.Contains(t, note, "mobile/src/App.tsx")
		require.NotContains(t, note, "Other.tsx")
	})
}

func TestBuildExploreToCodeContext_ReturnsEmptyWhenNoTargetsMatch(t *testing.T) {
	_, c, rootID, _ := setupExploreReadsFixture(t, []string{"a.go", "b.go", "c.go"})

	codeJob := &agentJob{
		id: "code-none", agent: config.AgentCode, sessionID: "code-s-none",
		rootSessionID: rootID,
	}
	require.Empty(t, c.buildWorkerContextNote(t.Context(), codeJob, nil, "Implement the new authentication handler"))

	codeJobMissing := &agentJob{
		id: "code-missing", agent: config.AgentCode, sessionID: "code-s-missing",
		rootSessionID: rootID, contextFrom: []string{"exp"},
	}
	require.Empty(t, c.buildWorkerContextNote(t.Context(), codeJobMissing, nil, "Edit nonexistent.go"))
}

func TestExecuteWorker_FollowUpAgentMessageContextFiles(t *testing.T) {
	env, c, rootID, _ := setupExploreReadsFixture(t, []string{"a.go", "b.go", "c.go"})

	codeCalls := make(chan SessionAgentCall, 2)
	codeAgent := newMockAgent("job-provider", 1024, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		codeCalls <- call
		return agentResultWithText("done"), nil
	})
	c.subAgents = map[string]SessionAgent{
		config.AgentCode: codeAgent,
	}

	codeJob, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          codeAgent,
		SessionID:      rootID,
		AgentMessageID: "m1",
		ToolCallID:     "code-call",
		Prompt:         "Implement feature in a.go",
		SessionTitle:   "Agent Session: code | impl",
		ContextFiles:   []string{"a.go"},
	}, config.AgentCode)
	require.NoError(t, err)

	var firstCall SessionAgentCall
	select {
	case firstCall = <-codeCalls:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for initial code worker call")
	}
	require.Contains(t, firstCall.ContextNote, "### a.go (lines 1-1)")
	require.NotContains(t, firstCall.ContextNote, "b.go")
	require.NotContains(t, firstCall.ContextNote, "c.go")
	require.True(t, codeJob.wait(t.Context(), 3*time.Second))

	msgInput, err := json.Marshal(agentMessageParams{
		Op:           "send",
		WorkerID:     codeJob.id,
		Text:         "Now update the helper",
		ContextFrom:  []string{"exp"},
		ContextFiles: []string{"b.go"},
	})
	require.NoError(t, err)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, rootID)
	resp, err := c.agentMessageTool().Run(ctx, fantasy.ToolCall{
		ID:    "msg-1",
		Name:  AgentMessageToolName,
		Input: string(msgInput),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	var followUpCall SessionAgentCall
	select {
	case followUpCall = <-codeCalls:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for follow-up code worker call")
	}
	require.Contains(t, followUpCall.ContextNote, "### b.go (lines 1-1)")
	require.NotContains(t, followUpCall.ContextNote, "a.go")
	require.NotContains(t, followUpCall.ContextNote, "c.go")
	require.True(t, codeJob.wait(t.Context(), 3*time.Second))
	_ = env
}
