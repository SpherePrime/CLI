package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/session"
)

type AgentWorker struct {
	ID              string `json:"id"`
	SessionID       string `json:"session_id"`
	ParentSessionID string `json:"parent_session_id"`
	RootSessionID   string `json:"root_session_id"`
	Name            string `json:"name"`
	Agent           string `json:"agent"`
	Task            string `json:"task"`
	Status          string `json:"status"`
	StartedAt       int64  `json:"started_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type AgentOrchestrator interface {
	AgentWorkers(context.Context, string) ([]AgentWorker, error)
	SendAgentWorkerMessage(context.Context, string, string, string) (AgentWorker, error)
	StopAgentWorker(context.Context, string, string) error
}

func (c *coordinator) teamRoot(ctx context.Context, sessionID string) (string, error) {
	seen := map[string]bool{}
	for sessionID != "" && !seen[sessionID] {
		seen[sessionID] = true
		s, err := c.sessions.Get(ctx, sessionID)
		if err != nil {
			return "", err
		}
		if s.ParentSessionID == "" {
			return s.ID, nil
		}
		sessionID = s.ParentSessionID
	}
	return "", errors.New("invalid session ancestry")
}

func (j *agentJob) worker() AgentWorker {
	j.mu.Lock()
	defer j.mu.Unlock()
	return AgentWorker{ID: j.id, SessionID: j.sessionID, ParentSessionID: j.parentSessionID, RootSessionID: j.rootSessionID, Name: j.name, Agent: j.agent, Task: j.prompt, Status: j.status, StartedAt: j.startedAt.Unix(), UpdatedAt: j.updatedAt.Unix()}
}

func (c *coordinator) AgentWorkers(ctx context.Context, sessionID string) ([]AgentWorker, error) {
	root, err := c.teamRoot(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	workers := []AgentWorker{}
	live := map[string]bool{}
	for _, job := range c.registrySnapshot() {
		worker := job.worker()
		if worker.RootSessionID == "" {
			worker.RootSessionID, _ = c.teamRoot(ctx, worker.ParentSessionID)
		}
		if worker.RootSessionID == root {
			workers = append(workers, worker)
			live[worker.SessionID] = true
		}
	}
	var sessions []session.Session
	if children, ok := c.sessions.(interface {
		ListChildren(context.Context, string) ([]session.Session, error)
	}); ok {
		sessions, err = children.ListChildren(ctx, root)
		if err != nil {
			return nil, err
		}
	} else if c.messages != nil {
		entries, err := c.messages.ListAllUserMessages(ctx)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			if seen[entry.SessionID] || !c.sessions.IsAgentToolSession(entry.SessionID) {
				continue
			}
			seen[entry.SessionID] = true
			if child, err := c.sessions.Get(ctx, entry.SessionID); err == nil {
				sessions = append(sessions, child)
			}
		}
	}
	for _, s := range sessions {
		if live[s.ID] || s.ParentSessionID == "" || !c.sessions.IsAgentToolSession(s.ID) {
			continue
		}
		childRoot, err := c.teamRoot(ctx, s.ParentSessionID)
		if err != nil || childRoot != root {
			continue
		}
		role, name := historicalWorkerTitle(s.Title)
		worker := AgentWorker{ID: s.ID, SessionID: s.ID, ParentSessionID: s.ParentSessionID, RootSessionID: root, Name: name, Agent: role, Status: "idle", StartedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
		if c.messages != nil {
			history, err := c.messages.List(ctx, s.ID)
			if err != nil {
				return nil, err
			}
			for _, entry := range history {
				if entry.Role == message.User {
					worker.Task = entry.Content().Text
					break
				}
			}
			if len(history) > 0 && history[len(history)-1].Role == message.User {
				worker.Status = "interrupted"
			}
		}
		workers = append(workers, worker)
	}
	return workers, nil
}

func historicalWorkerTitle(title string) (string, string) {
	value := strings.TrimPrefix(title, "Agent Session: ")
	role, name, found := strings.Cut(value, " | ")
	if !found {
		name = value
	}
	if role == "" || strings.ContainsAny(role, " \n") {
		role = "task"
	}
	return role, name
}

func (c *coordinator) teamWorker(ctx context.Context, source, workerID string) (AgentWorker, error) {
	workers, err := c.AgentWorkers(ctx, source)
	if err != nil {
		return AgentWorker{}, err
	}
	for _, worker := range workers {
		if worker.ID == workerID || worker.SessionID == workerID {
			return worker, nil
		}
	}
	return AgentWorker{}, errors.New("agent worker not found in this team")
}

func (c *coordinator) StopAgentWorker(ctx context.Context, source, workerID string) error {
	worker, err := c.teamWorker(ctx, source, workerID)
	if err != nil {
		return err
	}
	if job := c.findJob(worker.ID); job != nil {
		job.mu.Lock()
		job.pending = nil
		cancel := job.cancel
		job.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	return nil
}

func (c *coordinator) SendAgentWorkerMessage(ctx context.Context, source, workerID, text string) (AgentWorker, error) {
	if strings.TrimSpace(text) == "" {
		return AgentWorker{}, errors.New("message text is required")
	}
	worker, err := c.teamWorker(ctx, source, workerID)
	if err != nil {
		return AgentWorker{}, err
	}
	if c.cfg.Config().Options.DisableMultiAgent {
		return AgentWorker{}, errors.New("agent orchestration is disabled")
	}
	if source == worker.SessionID {
		return AgentWorker{}, errors.New("cannot send a worker a message from itself")
	}
	job := c.findJob(worker.ID)
	if job == nil {
		a, err := c.SubAgent(worker.Agent)
		if err != nil {
			return AgentWorker{}, err
		}
		c.jobMu.Lock()
		job = c.agentJobs[worker.ID]
		if job == nil {
			job = &agentJob{id: worker.ID, sessionID: worker.SessionID, parentSessionID: worker.ParentSessionID, rootSessionID: worker.RootSessionID, agent: worker.Agent, name: worker.Name, prompt: worker.Task, status: worker.Status, startedAt: time.Unix(worker.StartedAt, 0), updatedAt: time.Now(), hasRun: true, params: subAgentParams{Agent: a, SessionID: worker.ParentSessionID}}
			if c.agentJobs == nil {
				c.agentJobs = map[string]*agentJob{}
			}
			c.agentJobs[job.id] = job
		}
		c.jobMu.Unlock()
	}
	job.mu.Lock()
	job.pending = append(job.pending, fmt.Sprintf("Message from team member %s:\n%s", source, text))
	job.updatedAt = time.Now()
	if job.status != "running" && job.status != "queued" {
		jobCtx, cancel := context.WithCancel(c.workerContext())
		job.cancel = cancel
		job.done = make(chan struct{})
		job.status = "queued"
		job.queuedAt = time.Now()
		job.seen = false
		job.wake = true
		job.generation++
		job.mu.Unlock()
		go c.executeWorker(job, jobCtx)
	} else {
		job.mu.Unlock()
	}
	return job.worker(), nil
}
