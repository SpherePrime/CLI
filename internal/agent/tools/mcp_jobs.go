package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/csync"
)

var (
	MCPBackgroundAfter = 30 * time.Second
	runMCPTool         = mcp.RunToolWithProgress
)

type MCPJobWake func(sessionID, jobID, toolName, status, output string)

type MCPJobMetadata struct {
	Background bool   `json:"background"`
	JobID      string `json:"job_id"`
	StartTime  int64  `json:"start_time"`
}

type mcpJob struct {
	id        string
	mcpName   string
	toolName  string
	label     string
	sessionID string
	input     string
	startedAt time.Time
	cancel    context.CancelFunc
	done      chan struct{}

	mu         sync.Mutex
	finishedAt time.Time
	status     string
	result     mcp.ToolResult
	err        error
	background bool
	seen       bool
	killed     bool
}

var (
	mcpJobs   = csync.NewMap[string, *mcpJob]()
	mcpJobSeq atomic.Int64
)

func startMCPJob(ctx context.Context, tool *Tool, input, sessionID string) *mcpJob {
	pruneMCPJobs()
	jobCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	job := &mcpJob{
		id:        fmt.Sprintf("mcp-%d", mcpJobSeq.Add(1)),
		mcpName:   tool.mcpName,
		toolName:  tool.tool.Name,
		label:     tool.Name(),
		sessionID: sessionID,
		input:     input,
		startedAt: time.Now(),
		cancel:    cancel,
		done:      make(chan struct{}),
		status:    "running",
	}
	mcpJobs.Set(job.id, job)
	go func() {
		defer close(job.done)
		defer mcp.ForgetProgress(job.id)
		result, err := runMCPTool(jobCtx, tool.cfg, tool.mcpName, tool.tool.Name, input, sessionID, job.id)
		job.mu.Lock()
		defer job.mu.Unlock()
		job.finishedAt = time.Now()
		job.result, job.err = result, err
		switch {
		case job.killed:
			job.status = "canceled"
		case err != nil:
			job.status = "error"
		default:
			job.status = "done"
		}
	}()
	return job
}

func (j *mcpJob) kill() {
	j.mu.Lock()
	if j.status == "running" {
		j.killed = true
	}
	j.mu.Unlock()
	j.cancel()
}

func (j *mcpJob) snapshot() (status string, result mcp.ToolResult, err error, finishedAt time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status, j.result, j.err, j.finishedAt
}

func (j *mcpJob) moveToBackground() {
	j.mu.Lock()
	j.background = true
	j.mu.Unlock()
}

func (j *mcpJob) markSeen() {
	j.mu.Lock()
	j.seen = true
	j.mu.Unlock()
}

func (j *mcpJob) watch(wake MCPJobWake) {
	<-j.done
	if wake == nil {
		return
	}
	j.mu.Lock()
	skip := j.killed || j.seen
	status := j.status
	output := mcpJobText(j.result, j.err)
	j.mu.Unlock()
	if skip {
		return
	}
	if len(output) > backgroundWakeOutputLimit {
		output = output[:backgroundWakeOutputLimit] + " …"
	}
	wake(j.sessionID, j.id, j.label, status, output)
}

func mcpJobText(result mcp.ToolResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result.Type == "image" || result.Type == "media" {
		text := result.Content
		if text != "" {
			text += "\n"
		}
		return text + fmt.Sprintf("[%s result, %s]", result.Type, result.MediaType)
	}
	return result.Content
}

func MCPJobResultSeen(jobID string) bool {
	job, ok := mcpJobs.Get(jobID)
	if !ok {
		return false
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.seen
}

func sessionMCPJobs(sessionID string) []*mcpJob {
	var jobs []*mcpJob
	for _, job := range mcpJobs.Seq2() {
		if job.sessionID == sessionID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].startedAt.Before(jobs[b].startedAt) })
	return jobs
}

func pruneMCPJobs() {
	for id, job := range mcpJobs.Seq2() {
		job.mu.Lock()
		stale := job.status != "running" && time.Since(job.finishedAt) > time.Hour
		job.mu.Unlock()
		if stale {
			mcpJobs.Del(id)
		}
	}
}
