package customagents

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

const (
	defaultOrchestratorPrompt = `Ты главный координатор автономных агентных процессов.
Твои обязанности:
1. Декомпозировать поставленную задачу на чёткие, проверяемые подзадачи.
2. Подбирать подходящих специализированных агентов или формулировать требования к новым исполнителям.
3. Фиксировать промежуточные результаты, решения и зависимости в общей доске контекста (blackboard).
4. Проверять соответствие итогового результата критериям приёмки перед завершением цепочки.
Отвечай структурированно, по делу и опирайся только на подтверждённые факты.`

	defaultVerifierPrompt = `Ты строгий независимый верификатор результатов.
Твоя задача — тщательно проверить результат исполнителя на соответствие заданным критериям приёмки и фактической полноте.
Правила оценки:
1. Проверь каждый пункт критериев приёмки и исходной постановки задачи.
2. Если есть хотя бы один пробел, противоречие, пропуск или неподтверждённое утверждение — отклоняй результат.
3. Первая строка ответа ОБЯЗАНА содержать ровно один вердикт:
   VERDICT: PASS
   или
   VERDICT: FAIL
4. При VERDICT: FAIL приведи нумерованный список конкретных дефектов и что нужно исправить.`
)

type Store struct {
	db           *sql.DB
	dataDir      string
	memoriesDir  string
	globalMemory *GlobalMemoryStore
	mu           sync.Mutex
}

func DefaultDataDir() string {
	return filepath.Join(config.GlobalDataDir(), "custom-agents")
}

func OpenStore(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = DefaultDataDir()
	}
	memoriesDir := filepath.Join(dataDir, "memories")
	if err := os.MkdirAll(memoriesDir, 0o700); err != nil {
		return nil, fmt.Errorf("create memories dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "custom_agents.db")
	conn, err := db.OpenFile(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open custom agents db: %w", err)
	}

	if err := initStoreSchema(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("init custom agents schema: %w", err)
	}

	return &Store{
		db:           conn,
		dataDir:      dataDir,
		memoriesDir:  memoriesDir,
		globalMemory: NewGlobalMemoryStore(dataDir, WithUseConfigSoul(!testing.Testing())),
	}, nil
}

func (s *Store) GlobalMemory() *GlobalMemoryStore {
	return s.globalMemory
}

func initStoreSchema(conn *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS custom_agents (
			id TEXT PRIMARY KEY,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'worker',
			system_prompt TEXT NOT NULL DEFAULT '',
			memory TEXT NOT NULL DEFAULT '',
			model_mode TEXT NOT NULL DEFAULT 'auto',
			provider_id TEXT NOT NULL DEFAULT '',
			model_id TEXT NOT NULL DEFAULT '',
			reasoning_mode TEXT NOT NULL DEFAULT '',
			auto_model_hint TEXT NOT NULL DEFAULT '',
			allowed_tools TEXT NOT NULL DEFAULT '[]',
			workspace_path TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_by TEXT NOT NULL DEFAULT 'user',
			trigger_type TEXT NOT NULL DEFAULT 'manual',
			interval_minutes INTEGER NOT NULL DEFAULT 0,
			cron_expr TEXT NOT NULL DEFAULT '',
			monitor_kind TEXT NOT NULL DEFAULT '',
			monitor_target TEXT NOT NULL DEFAULT '',
			monitor_last_hash TEXT NOT NULL DEFAULT '',
			monitor_last_snapshot TEXT NOT NULL DEFAULT '',
			acceptance_criteria TEXT NOT NULL DEFAULT '',
			deterministic_checks TEXT NOT NULL DEFAULT '[]',
			verifier_agent_id TEXT NOT NULL DEFAULT '',
			next_agent_id TEXT NOT NULL DEFAULT '',
			max_rework_attempts INTEGER NOT NULL DEFAULT 3,
			strict_verify INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'idle',
			last_error TEXT NOT NULL DEFAULT '',
			last_run_at INTEGER NOT NULL DEFAULT 0,
			next_run_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS custom_agent_runs (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			agent_name TEXT NOT NULL DEFAULT '',
			agent_role TEXT NOT NULL DEFAULT '',
			chain_id TEXT NOT NULL DEFAULT '',
			parent_run_id TEXT NOT NULL DEFAULT '',
			trigger_reason TEXT NOT NULL DEFAULT '',
			input_prompt TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			attempt INTEGER NOT NULL DEFAULT 1,
			resolved_provider TEXT NOT NULL DEFAULT '',
			resolved_model TEXT NOT NULL DEFAULT '',
			resolved_reasoning TEXT NOT NULL DEFAULT '',
			auto_model_reason TEXT NOT NULL DEFAULT '',
			output_text TEXT NOT NULL DEFAULT '',
			verifier_verdict TEXT NOT NULL DEFAULT '',
			verifier_feedback TEXT NOT NULL DEFAULT '',
			deterministic_report TEXT NOT NULL DEFAULT '',
			monitor_diff TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '[]',
			error TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL DEFAULT 0,
			finished_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS custom_agent_blackboard (
			id TEXT PRIMARY KEY,
			chain_id TEXT NOT NULL,
			run_id TEXT NOT NULL DEFAULT '',
			from_agent_id TEXT NOT NULL DEFAULT '',
			from_agent_name TEXT NOT NULL DEFAULT '',
			to_agent_id TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL DEFAULT '',
			payload TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open',
			created_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_custom_agents_slug ON custom_agents(slug);`,
		`CREATE INDEX IF NOT EXISTS idx_custom_agents_enabled_next ON custom_agents(enabled, trigger_type, next_run_at);`,
		`CREATE INDEX IF NOT EXISTS idx_custom_agent_runs_agent_started ON custom_agent_runs(agent_id, started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_custom_agent_runs_chain ON custom_agent_runs(chain_id, started_at ASC);`,
		`CREATE INDEX IF NOT EXISTS idx_custom_agent_blackboard_chain ON custom_agent_blackboard(chain_id, created_at ASC);`,
	}

	for _, stmt := range stmts {
		if _, err := conn.Exec(stmt); err != nil {
			return err
		}
	}
	if err := ensureCustomAgentsColumn(conn, "strict_verify", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func ensureCustomAgentsColumn(conn *sql.DB, column, definition string) error {
	rows, err := conn.Query(`PRAGMA table_info(custom_agents)`)
	if err != nil {
		return fmt.Errorf("inspect custom_agents schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			return fmt.Errorf("scan custom_agents column: %w", err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`ALTER TABLE custom_agents ADD COLUMN %s %s`, column, definition))
	if err != nil {
		return fmt.Errorf("add column %s: %w", column, err)
	}
	return nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) EnsureDefaults(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM custom_agents`).Scan(&count); err != nil {
		return fmt.Errorf("count custom agents: %w", err)
	}
	if count > 0 {
		return nil
	}

	defaults := []CustomAgent{
		{
			Slug:              "orchestrator",
			Name:              "Главный координатор",
			Description:       "Принимает задачи, создаёт специализированных агентов, подбирает модели и собирает результат",
			Role:              RoleOrchestrator,
			SystemPrompt:      defaultOrchestratorPrompt,
			ModelMode:         ModelModeAuto,
			Enabled:           true,
			CreatedBy:         "system",
			TriggerType:       TriggerManual,
			MaxReworkAttempts: 3,
			Status:            StatusIdle,
		},
		{
			Slug:              "strict-verifier",
			Name:              "Строгий верификатор",
			Description:       "Проверяет результаты воркеров по жёстким критериям и отклоняет недоработки",
			Role:              RoleVerifier,
			SystemPrompt:      defaultVerifierPrompt,
			ModelMode:         ModelModeAuto,
			Enabled:           true,
			CreatedBy:         "system",
			TriggerType:       TriggerManual,
			MaxReworkAttempts: 1,
			Status:            StatusIdle,
		},
	}

	for _, agent := range defaults {
		if _, err := s.CreateAgent(ctx, agent); err != nil {
			return fmt.Errorf("seed default agent %s: %w", agent.Slug, err)
		}
	}
	return nil
}

func (s *Store) CreateAgent(ctx context.Context, agent CustomAgent) (CustomAgent, error) {
	now := time.Now().Unix()
	if agent.ID == "" {
		agent.ID = uuid.NewString()
	}
	agent.Slug = normalizeSlug(agent.Slug, agent.Name, agent.ID)
	if agent.Role == "" {
		agent.Role = RoleWorker
	}
	if agent.ModelMode == "" {
		agent.ModelMode = ModelModeAuto
	}
	if agent.CreatedBy == "" {
		agent.CreatedBy = "user"
	}
	if agent.TriggerType == "" {
		agent.TriggerType = TriggerManual
	}
	if agent.MaxReworkAttempts <= 0 {
		agent.MaxReworkAttempts = 3
	}
	if agent.Status == "" {
		if agent.Enabled {
			agent.Status = StatusIdle
		} else {
			agent.Status = StatusPaused
		}
	}
	if agent.CreatedAt == 0 {
		agent.CreatedAt = now
	}
	if agent.UpdatedAt == 0 {
		agent.UpdatedAt = now
	}
	if agent.TriggerType == TriggerCron && agent.NextRunAt == 0 && strings.TrimSpace(agent.CronExpr) != "" {
		if sched, err := ParseCron(agent.CronExpr); err == nil {
			if next := sched.NextAfter(time.Unix(now, 0)); !next.IsZero() {
				agent.NextRunAt = next.Unix()
			}
		}
	}

	toolsJSON, err := json.Marshal(nonNilStrings(agent.AllowedTools))
	if err != nil {
		return CustomAgent{}, fmt.Errorf("marshal allowed_tools: %w", err)
	}
	checksJSON, err := json.Marshal(nonNilChecks(agent.DeterministicChecks))
	if err != nil {
		return CustomAgent{}, fmt.Errorf("marshal deterministic_checks: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO custom_agents (
			id, slug, name, description, role, system_prompt, memory,
			model_mode, provider_id, model_id, reasoning_mode, auto_model_hint,
			allowed_tools, workspace_path, enabled, created_by,
			trigger_type, interval_minutes, cron_expr,
			monitor_kind, monitor_target, monitor_last_hash, monitor_last_snapshot,
			acceptance_criteria, deterministic_checks, verifier_agent_id, next_agent_id,
			max_rework_attempts, strict_verify, status, last_error, last_run_at, next_run_at,
			created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?
		)`,
		agent.ID, agent.Slug, agent.Name, agent.Description, agent.Role, agent.SystemPrompt, agent.Memory,
		agent.ModelMode, agent.ProviderID, agent.ModelID, agent.ReasoningMode, agent.AutoModelHint,
		string(toolsJSON), agent.WorkspacePath, boolToInt(agent.Enabled), agent.CreatedBy,
		agent.TriggerType, agent.IntervalMinutes, agent.CronExpr,
		agent.MonitorKind, agent.MonitorTarget, agent.MonitorLastHash, agent.MonitorLastSnapshot,
		agent.AcceptanceCriteria, string(checksJSON), agent.VerifierAgentID, agent.NextAgentID,
		agent.MaxReworkAttempts, boolToInt(agent.StrictVerify), agent.Status, agent.LastError, agent.LastRunAt, agent.NextRunAt,
		agent.CreatedAt, agent.UpdatedAt,
	)
	if err != nil {
		return CustomAgent{}, fmt.Errorf("insert custom agent: %w", err)
	}

	if agent.Memory != "" {
		if err := s.writeMemoryFileLocked(agent.ID, agent.Memory); err != nil {
			return CustomAgent{}, err
		}
	}

	return agent, nil
}

func (s *Store) GetAgent(ctx context.Context, id string) (CustomAgent, error) {
	row := s.db.QueryRowContext(ctx, selectAgentCols+` WHERE id = ?`, id)
	return scanAgentRow(row)
}

func (s *Store) GetAgentBySlug(ctx context.Context, slug string) (CustomAgent, error) {
	row := s.db.QueryRowContext(ctx, selectAgentCols+` WHERE slug = ?`, slug)
	return scanAgentRow(row)
}

func (s *Store) ListAgents(ctx context.Context) ([]CustomAgent, error) {
	rows, err := s.db.QueryContext(ctx, selectAgentCols+` ORDER BY created_at ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list custom agents: %w", err)
	}
	defer rows.Close()

	var agents []CustomAgent
	for rows.Next() {
		a, err := scanAgentRow(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return agents, nil
}

func (s *Store) UpdateAgent(ctx context.Context, agent CustomAgent) (CustomAgent, error) {
	if agent.ID == "" {
		return CustomAgent{}, errors.New("agent id is required")
	}
	agent.Slug = normalizeSlug(agent.Slug, agent.Name, agent.ID)
	if agent.Role == "" {
		agent.Role = RoleWorker
	}
	if agent.ModelMode == "" {
		agent.ModelMode = ModelModeAuto
	}
	if agent.TriggerType == "" {
		agent.TriggerType = TriggerManual
	}
	if agent.MaxReworkAttempts <= 0 {
		agent.MaxReworkAttempts = 3
	}
	if agent.Status == "" {
		if agent.Enabled {
			agent.Status = StatusIdle
		} else {
			agent.Status = StatusPaused
		}
	}
	agent.UpdatedAt = time.Now().Unix()

	toolsJSON, err := json.Marshal(nonNilStrings(agent.AllowedTools))
	if err != nil {
		return CustomAgent{}, fmt.Errorf("marshal allowed_tools: %w", err)
	}
	checksJSON, err := json.Marshal(nonNilChecks(agent.DeterministicChecks))
	if err != nil {
		return CustomAgent{}, fmt.Errorf("marshal deterministic_checks: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `
		UPDATE custom_agents SET
			slug = ?, name = ?, description = ?, role = ?, system_prompt = ?, memory = ?,
			model_mode = ?, provider_id = ?, model_id = ?, reasoning_mode = ?, auto_model_hint = ?,
			allowed_tools = ?, workspace_path = ?, enabled = ?, created_by = ?,
			trigger_type = ?, interval_minutes = ?, cron_expr = ?,
			monitor_kind = ?, monitor_target = ?, monitor_last_hash = ?, monitor_last_snapshot = ?,
			acceptance_criteria = ?, deterministic_checks = ?, verifier_agent_id = ?, next_agent_id = ?,
			max_rework_attempts = ?, strict_verify = ?, status = ?, last_error = ?, last_run_at = ?, next_run_at = ?,
			updated_at = ?
		WHERE id = ?`,
		agent.Slug, agent.Name, agent.Description, agent.Role, agent.SystemPrompt, agent.Memory,
		agent.ModelMode, agent.ProviderID, agent.ModelID, agent.ReasoningMode, agent.AutoModelHint,
		string(toolsJSON), agent.WorkspacePath, boolToInt(agent.Enabled), agent.CreatedBy,
		agent.TriggerType, agent.IntervalMinutes, agent.CronExpr,
		agent.MonitorKind, agent.MonitorTarget, agent.MonitorLastHash, agent.MonitorLastSnapshot,
		agent.AcceptanceCriteria, string(checksJSON), agent.VerifierAgentID, agent.NextAgentID,
		agent.MaxReworkAttempts, boolToInt(agent.StrictVerify), agent.Status, agent.LastError, agent.LastRunAt, agent.NextRunAt,
		agent.UpdatedAt, agent.ID,
	)
	if err != nil {
		return CustomAgent{}, fmt.Errorf("update custom agent: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return CustomAgent{}, err
	}
	if affected == 0 {
		return CustomAgent{}, sql.ErrNoRows
	}

	if err := s.writeMemoryFileLocked(agent.ID, agent.Memory); err != nil {
		return CustomAgent{}, err
	}

	return agent, nil
}

func (s *Store) UpdateAgentRuntimeState(ctx context.Context, agent CustomAgent) error {
	if agent.ID == "" {
		return errors.New("agent id is required")
	}
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `
		UPDATE custom_agents SET
			status = ?, last_error = ?, last_run_at = ?, next_run_at = ?,
			monitor_last_hash = ?, monitor_last_snapshot = ?, updated_at = ?
		WHERE id = ?`,
		agent.Status, agent.LastError, agent.LastRunAt, agent.NextRunAt,
		agent.MonitorLastHash, agent.MonitorLastSnapshot, now, agent.ID,
	)
	if err != nil {
		return fmt.Errorf("update custom agent runtime state: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteAgent(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM custom_agents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete custom agent: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	memPath := s.memoryPath(id)
	if err := os.Remove(memPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove memory file: %w", err)
	}
	return nil
}

func (s *Store) ReadMemory(ctx context.Context, agentID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readMemoryLocked(ctx, agentID)
}

func (s *Store) readMemoryLocked(ctx context.Context, agentID string) (string, error) {
	data, err := os.ReadFile(s.memoryPath(agentID))
	if err == nil {
		return string(data), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read memory file: %w", err)
	}

	var mem string
	err = s.db.QueryRowContext(ctx, `SELECT memory FROM custom_agents WHERE id = ?`, agentID).Scan(&mem)
	if err != nil {
		return "", err
	}
	return mem, nil
}

func (s *Store) WriteMemory(ctx context.Context, agentID, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeMemoryLocked(ctx, agentID, content)
}

func (s *Store) writeMemoryLocked(ctx context.Context, agentID, content string) error {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `UPDATE custom_agents SET memory = ?, updated_at = ? WHERE id = ?`, content, now, agentID)
	if err != nil {
		return fmt.Errorf("update agent memory: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return s.writeMemoryFileLocked(agentID, content)
}

func (s *Store) AppendMemory(ctx context.Context, agentID, entry string) error {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readMemoryLocked(ctx, agentID)
	if err != nil {
		return err
	}
	existing = strings.TrimSpace(existing)
	var updated string
	if existing == "" {
		updated = entry
	} else {
		updated = existing + "\n\n" + entry
	}
	return s.writeMemoryLocked(ctx, agentID, updated)
}

func (s *Store) memoryPath(agentID string) string {
	safeID := strings.ReplaceAll(agentID, string(filepath.Separator), "_")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	return filepath.Join(s.memoriesDir, safeID+".md")
}

func (s *Store) writeMemoryFileLocked(agentID, content string) error {
	target := s.memoryPath(agentID)
	tmp, err := os.CreateTemp(s.memoriesDir, filepath.Base(target)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp memory file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp memory file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp memory file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp memory file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename memory file: %w", err)
	}
	return nil
}

func (s *Store) CreateRun(ctx context.Context, run CustomAgentRun) (CustomAgentRun, error) {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.ChainID == "" {
		run.ChainID = uuid.NewString()
	}
	if run.Attempt <= 0 {
		run.Attempt = 1
	}
	if run.StartedAt == 0 {
		run.StartedAt = time.Now().Unix()
	}
	if run.Status == "" {
		run.Status = RunRunning
	}

	stepsJSON, err := json.Marshal(nonNilSteps(run.Steps))
	if err != nil {
		return CustomAgentRun{}, fmt.Errorf("marshal steps: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO custom_agent_runs (
			id, agent_id, agent_name, agent_role, chain_id, parent_run_id,
			trigger_reason, input_prompt, status, attempt,
			resolved_provider, resolved_model, resolved_reasoning, auto_model_reason,
			output_text, verifier_verdict, verifier_feedback, deterministic_report,
			monitor_diff, steps, error, started_at, finished_at
		) VALUES (
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?
		)`,
		run.ID, run.AgentID, run.AgentName, run.AgentRole, run.ChainID, run.ParentRunID,
		run.TriggerReason, run.InputPrompt, run.Status, run.Attempt,
		run.ResolvedProvider, run.ResolvedModel, run.ResolvedReasoning, run.AutoModelReason,
		run.OutputText, run.VerifierVerdict, run.VerifierFeedback, run.DeterministicReport,
		run.MonitorDiff, string(stepsJSON), run.Error, run.StartedAt, run.FinishedAt,
	)
	if err != nil {
		return CustomAgentRun{}, fmt.Errorf("insert custom agent run: %w", err)
	}
	return run, nil
}

func (s *Store) UpdateRun(ctx context.Context, run CustomAgentRun) error {
	stepsJSON, err := json.Marshal(nonNilSteps(run.Steps))
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE custom_agent_runs SET
			status = ?, attempt = ?,
			resolved_provider = ?, resolved_model = ?, resolved_reasoning = ?, auto_model_reason = ?,
			output_text = ?, verifier_verdict = ?, verifier_feedback = ?, deterministic_report = ?,
			monitor_diff = ?, steps = ?, error = ?, finished_at = ?
		WHERE id = ?`,
		run.Status, run.Attempt,
		run.ResolvedProvider, run.ResolvedModel, run.ResolvedReasoning, run.AutoModelReason,
		run.OutputText, run.VerifierVerdict, run.VerifierFeedback, run.DeterministicReport,
		run.MonitorDiff, string(stepsJSON), run.Error, run.FinishedAt,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("update custom agent run: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, id string) (CustomAgentRun, error) {
	row := s.db.QueryRowContext(ctx, selectRunCols+` WHERE id = ?`, id)
	return scanRunRow(row)
}

func (s *Store) ListRuns(ctx context.Context, agentID string, limit int) ([]CustomAgentRun, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(agentID) == "" {
		rows, err = s.db.QueryContext(ctx, selectRunCols+` ORDER BY started_at DESC, rowid DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, selectRunCols+` WHERE agent_id = ? ORDER BY started_at DESC, rowid DESC LIMIT ?`, agentID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list custom agent runs: %w", err)
	}
	defer rows.Close()

	var runs []CustomAgentRun
	for rows.Next() {
		r, err := scanRunRow(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *Store) PostBlackboard(ctx context.Context, entry BlackboardEntry) (BlackboardEntry, error) {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.Status == "" {
		entry.Status = "open"
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO custom_agent_blackboard (
			id, chain_id, run_id, from_agent_id, from_agent_name,
			to_agent_id, topic, payload, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ChainID, entry.RunID, entry.FromAgentID, entry.FromAgentName,
		entry.ToAgentID, entry.Topic, entry.Payload, entry.Status, entry.CreatedAt,
	)
	if err != nil {
		return BlackboardEntry{}, fmt.Errorf("post blackboard: %w", err)
	}
	return entry, nil
}

func (s *Store) ListBlackboard(ctx context.Context, chainID string, limit int) ([]BlackboardEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(chainID) == "" {
		rows, err = s.db.QueryContext(ctx, selectBlackboardCols+` ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, selectBlackboardCols+` WHERE chain_id = ? ORDER BY created_at ASC, rowid ASC LIMIT ?`, chainID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list blackboard: %w", err)
	}
	defer rows.Close()

	var entries []BlackboardEntry
	for rows.Next() {
		var e BlackboardEntry
		if err := rows.Scan(
			&e.ID, &e.ChainID, &e.RunID, &e.FromAgentID, &e.FromAgentName,
			&e.ToAgentID, &e.Topic, &e.Payload, &e.Status, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Store) MarkBlackboardConsumed(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE custom_agent_blackboard SET status = 'consumed' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mark blackboard consumed: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

const selectAgentCols = `
	SELECT
		id, slug, name, description, role, system_prompt, memory,
		model_mode, provider_id, model_id, reasoning_mode, auto_model_hint,
		allowed_tools, workspace_path, enabled, created_by,
		trigger_type, interval_minutes, cron_expr,
		monitor_kind, monitor_target, monitor_last_hash, monitor_last_snapshot,
		acceptance_criteria, deterministic_checks, verifier_agent_id, next_agent_id,
		max_rework_attempts, strict_verify, status, last_error, last_run_at, next_run_at,
		created_at, updated_at
	FROM custom_agents`

const selectRunCols = `
	SELECT
		id, agent_id, agent_name, agent_role, chain_id, parent_run_id,
		trigger_reason, input_prompt, status, attempt,
		resolved_provider, resolved_model, resolved_reasoning, auto_model_reason,
		output_text, verifier_verdict, verifier_feedback, deterministic_report,
		monitor_diff, steps, error, started_at, finished_at
	FROM custom_agent_runs`

const selectBlackboardCols = `
	SELECT
		id, chain_id, run_id, from_agent_id, from_agent_name,
		to_agent_id, topic, payload, status, created_at
	FROM custom_agent_blackboard`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgentRow(scanner rowScanner) (CustomAgent, error) {
	var a CustomAgent
	var toolsJSON string
	var checksJSON string
	var enabledInt int
	var strictVerifyInt int

	err := scanner.Scan(
		&a.ID, &a.Slug, &a.Name, &a.Description, &a.Role, &a.SystemPrompt, &a.Memory,
		&a.ModelMode, &a.ProviderID, &a.ModelID, &a.ReasoningMode, &a.AutoModelHint,
		&toolsJSON, &a.WorkspacePath, &enabledInt, &a.CreatedBy,
		&a.TriggerType, &a.IntervalMinutes, &a.CronExpr,
		&a.MonitorKind, &a.MonitorTarget, &a.MonitorLastHash, &a.MonitorLastSnapshot,
		&a.AcceptanceCriteria, &checksJSON, &a.VerifierAgentID, &a.NextAgentID,
		&a.MaxReworkAttempts, &strictVerifyInt, &a.Status, &a.LastError, &a.LastRunAt, &a.NextRunAt,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return CustomAgent{}, err
	}
	a.Enabled = enabledInt != 0
	a.StrictVerify = strictVerifyInt != 0
	if toolsJSON != "" {
		if err := json.Unmarshal([]byte(toolsJSON), &a.AllowedTools); err != nil {
			return CustomAgent{}, fmt.Errorf("unmarshal allowed_tools: %w", err)
		}
	}
	if checksJSON != "" {
		if err := json.Unmarshal([]byte(checksJSON), &a.DeterministicChecks); err != nil {
			return CustomAgent{}, fmt.Errorf("unmarshal deterministic_checks: %w", err)
		}
	}
	return a, nil
}

func scanRunRow(scanner rowScanner) (CustomAgentRun, error) {
	var r CustomAgentRun
	var stepsJSON string

	err := scanner.Scan(
		&r.ID, &r.AgentID, &r.AgentName, &r.AgentRole, &r.ChainID, &r.ParentRunID,
		&r.TriggerReason, &r.InputPrompt, &r.Status, &r.Attempt,
		&r.ResolvedProvider, &r.ResolvedModel, &r.ResolvedReasoning, &r.AutoModelReason,
		&r.OutputText, &r.VerifierVerdict, &r.VerifierFeedback, &r.DeterministicReport,
		&r.MonitorDiff, &stepsJSON, &r.Error, &r.StartedAt, &r.FinishedAt,
	)
	if err != nil {
		return CustomAgentRun{}, err
	}
	if stepsJSON != "" {
		if err := json.Unmarshal([]byte(stepsJSON), &r.Steps); err != nil {
			return CustomAgentRun{}, fmt.Errorf("unmarshal steps: %w", err)
		}
	}
	return r, nil
}

func normalizeSlug(slug, name, id string) string {
	raw := strings.TrimSpace(slug)
	if raw == "" {
		raw = strings.TrimSpace(name)
	}
	raw = strings.ToLower(raw)
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		short := id
		if len(short) > 8 {
			short = short[:8]
		}
		if short == "" {
			short = uuid.NewString()[:8]
		}
		return "agent-" + short
	}
	return out
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nonNilChecks(in []DeterministicCheck) []DeterministicCheck {
	if in == nil {
		return []DeterministicCheck{}
	}
	return in
}

func nonNilSteps(in []RunStep) []RunStep {
	if in == nil {
		return []RunStep{}
	}
	return in
}
