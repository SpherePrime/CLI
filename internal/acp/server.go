package acp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/version"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

type SessionBackend interface {
	CreateWorkspace(args proto.Workspace) (proto.Workspace, error)
	AttachClient(workspaceID, clientID string) error
	DetachClient(workspaceID, clientID string)
	InitAgent(ctx context.Context, workspaceID string, interactive bool) error
	UpdateAgent(ctx context.Context, workspaceID string) error
	SetMainAgent(workspaceID, agentID string) error
	GetWorkspaceConfig(workspaceID string) (*config.Config, error)
	GetAgentInfo(workspaceID string) (proto.AgentInfo, error)
	CreateSession(ctx context.Context, workspaceID, title string) (session.Session, error)
	GetSession(ctx context.Context, workspaceID, sessionID string) (session.Session, error)
	ListSessions(ctx context.Context, workspaceID string) ([]session.Session, error)
	ListSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error)
	SendMessage(workspaceID string, msg proto.AgentMessage) error
	CancelSession(workspaceID, sessionID string) error
	GrantPermission(workspaceID string, req proto.PermissionGrant) (bool, error)
	UpdateSessionModel(workspaceID, sessionID string, model *config.SelectedModel) error
	SubscribeEvents(ctx context.Context, workspaceID string) (<-chan pubsub.Event[tea.Msg], error)
}

type BackendAdapter struct {
	Backend *backend.Backend
}

func (a *BackendAdapter) CreateWorkspace(args proto.Workspace) (proto.Workspace, error) {
	_, ws, err := a.Backend.CreateWorkspace(args)
	return ws, err
}

func (a *BackendAdapter) AttachClient(workspaceID, clientID string) error {
	return a.Backend.AttachClient(workspaceID, clientID)
}

func (a *BackendAdapter) DetachClient(workspaceID, clientID string) {
	a.Backend.DetachClient(workspaceID, clientID)
}

func (a *BackendAdapter) InitAgent(ctx context.Context, workspaceID string, interactive bool) error {
	return a.Backend.InitAgent(ctx, workspaceID, interactive)
}

func (a *BackendAdapter) UpdateAgent(ctx context.Context, workspaceID string) error {
	return a.Backend.UpdateAgent(ctx, workspaceID)
}

func (a *BackendAdapter) SetMainAgent(workspaceID, agentID string) error {
	return a.Backend.SetMainAgent(workspaceID, agentID)
}

func (a *BackendAdapter) GetWorkspaceConfig(workspaceID string) (*config.Config, error) {
	return a.Backend.GetWorkspaceConfig(workspaceID)
}

func (a *BackendAdapter) GetAgentInfo(workspaceID string) (proto.AgentInfo, error) {
	return a.Backend.GetAgentInfo(workspaceID)
}

func (a *BackendAdapter) CreateSession(ctx context.Context, workspaceID, title string) (session.Session, error) {
	return a.Backend.CreateSession(ctx, workspaceID, title)
}

func (a *BackendAdapter) GetSession(ctx context.Context, workspaceID, sessionID string) (session.Session, error) {
	return a.Backend.GetSession(ctx, workspaceID, sessionID)
}

func (a *BackendAdapter) ListSessions(ctx context.Context, workspaceID string) ([]session.Session, error) {
	return a.Backend.ListSessions(ctx, workspaceID)
}

func (a *BackendAdapter) ListSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	return a.Backend.ListSessionMessages(ctx, workspaceID, sessionID)
}

func (a *BackendAdapter) SendMessage(workspaceID string, msg proto.AgentMessage) error {
	return a.Backend.SendMessage(workspaceID, msg)
}

func (a *BackendAdapter) CancelSession(workspaceID, sessionID string) error {
	return a.Backend.CancelSession(workspaceID, sessionID)
}

func (a *BackendAdapter) GrantPermission(workspaceID string, req proto.PermissionGrant) (bool, error) {
	return a.Backend.GrantPermission(workspaceID, req)
}

func (a *BackendAdapter) UpdateSessionModel(workspaceID, sessionID string, model *config.SelectedModel) error {
	return a.Backend.UpdateSessionModel(workspaceID, sessionID, model)
}

func (a *BackendAdapter) SubscribeEvents(ctx context.Context, workspaceID string) (<-chan pubsub.Event[tea.Msg], error) {
	return a.Backend.SubscribeEvents(ctx, workspaceID)
}

type ServerOptions struct {
	DefaultCwd        string
	DataDir           string
	Debug             bool
	YOLO              bool
	AuthPollInterval  time.Duration
	CancelWaitTimeout time.Duration
}

type activePromptRun struct {
	runID     string
	cancel    context.CancelFunc
	cancelled atomic.Bool

	permMu       sync.Mutex
	pendingPerms map[string]context.CancelFunc
}

func newActivePromptRun(runID string, cancel context.CancelFunc) *activePromptRun {
	return &activePromptRun{
		runID:        runID,
		cancel:       cancel,
		pendingPerms: make(map[string]context.CancelFunc),
	}
}

func (r *activePromptRun) addPermission(id string, cancel context.CancelFunc) {
	r.permMu.Lock()
	r.pendingPerms[id] = cancel
	r.permMu.Unlock()
}

func (r *activePromptRun) removePermission(id string) {
	r.permMu.Lock()
	delete(r.pendingPerms, id)
	r.permMu.Unlock()
}

func (r *activePromptRun) cancelAllPermissions() {
	r.permMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(r.pendingPerms))
	for id, fn := range r.pendingPerms {
		cancels = append(cancels, fn)
		delete(r.pendingPerms, id)
	}
	r.permMu.Unlock()
	for _, fn := range cancels {
		if fn != nil {
			fn()
		}
	}
}

func (r *activePromptRun) markCancelled() {
	r.cancelled.Store(true)
}

func (r *activePromptRun) isCancelled() bool {
	return r.cancelled.Load()
}

func (r *activePromptRun) cancelRun() {
	r.markCancelled()
	r.cancelAllPermissions()
	if r.cancel != nil {
		r.cancel()
	}
}

type sessionState struct {
	id          string
	workspaceID string
	cwd         string

	mu            sync.Mutex
	mode          string
	reasoningMode string
	activeRun     *activePromptRun
}

func (st *sessionState) getConfigState() (string, string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	mode := st.mode
	if mode == "" {
		mode = ModeCoder
	}
	return mode, st.reasoningMode
}

func (st *sessionState) setMode(mode string) {
	st.mu.Lock()
	st.mode = mode
	st.mu.Unlock()
}

func (st *sessionState) setReasoningMode(reasoning string) {
	st.mu.Lock()
	st.reasoningMode = reasoning
	st.mu.Unlock()
}

func (st *sessionState) setActiveRun(run *activePromptRun) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.activeRun != nil {
		return false
	}
	st.activeRun = run
	return true
}

func (st *sessionState) clearActiveRun(run *activePromptRun) {
	st.mu.Lock()
	if st.activeRun == run {
		st.activeRun = nil
	}
	st.mu.Unlock()
}

func (st *sessionState) cancelActiveRun() {
	st.mu.Lock()
	run := st.activeRun
	st.mu.Unlock()
	if run != nil {
		run.cancelRun()
	}
}

type Server struct {
	backend           SessionBackend
	account           AccountService
	transport         *Transport
	opts              ServerOptions
	clientID          string
	authPollInterval  time.Duration
	cancelWaitTimeout time.Duration

	wsInitMu sync.Mutex

	mu          sync.Mutex
	initialized bool
	clientCaps  ClientCapabilities
	workspaces  map[string]string
	attached    []string
	sessions    map[string]*sessionState
}

func NewServer(b SessionBackend, acc AccountService, r io.Reader, w io.Writer, opts ServerOptions) *Server {
	return &Server{
		backend:           b,
		account:           acc,
		transport:         NewTransport(r, w),
		opts:              opts,
		clientID:          uuid.NewString(),
		authPollInterval:  opts.AuthPollInterval,
		cancelWaitTimeout: opts.CancelWaitTimeout,
		workspaces:        make(map[string]string),
		sessions:          make(map[string]*sessionState),
	}
}

func (s *Server) Serve(ctx context.Context) error {
	defer s.detachAllWorkspaces()
	return s.transport.Serve(ctx, s.handleRequest, s.handleNotification)
}

func (s *Server) detachAllWorkspaces() {
	s.mu.Lock()
	wsIDs := append([]string(nil), s.attached...)
	s.attached = nil
	clear(s.workspaces)
	s.mu.Unlock()

	for _, wsID := range wsIDs {
		s.backend.DetachClient(wsID, s.clientID)
	}
}

func (s *Server) isInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialized
}

func (s *Server) getSession(sessionID string) (*sessionState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	return sess, ok
}

func (s *Server) upsertSession(sessionID, workspaceID, cwd string) *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.sessions[sessionID]; ok {
		return existing
	}
	sess := &sessionState{
		id:          sessionID,
		workspaceID: workspaceID,
		cwd:         cwd,
		mode:        ModeCoder,
	}
	s.sessions[sessionID] = sess
	return sess
}

func (s *Server) sendSessionUpdate(sessionID string, update any) error {
	return s.transport.Notify("session/update", SessionUpdateParams{
		SessionID: sessionID,
		Update:    update,
	})
}

func (s *Server) handleRequest(ctx context.Context, method string, rawParams json.RawMessage) (any, error) {
	if method == "initialize" {
		return s.handleInitialize(rawParams)
	}

	if !s.isInitialized() {
		return nil, NewRPCError(CodeInvalidRequest, "server not initialized; call initialize first")
	}

	switch method {
	case "authenticate":
		var params AuthenticateParams
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, err
		}
		if err := s.Authenticate(ctx, params.MethodID); err != nil {
			return nil, err
		}
		return AuthenticateResult{}, nil
	case "session/new":
		var params NewSessionParams
		if err := decodeOptionalParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handleSessionNew(ctx, params)
	case "session/load":
		var params LoadSessionParams
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handleSessionLoad(ctx, params)
	case "session/list":
		var params ListSessionsParams
		if err := decodeOptionalParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handleSessionList(ctx, params)
	case "session/prompt":
		var params PromptParams
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handlePrompt(ctx, params)
	case "session/set_mode":
		var params SetModeParams
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handleSetMode(ctx, params)
	case "session/set_model":
		var params SetModelParams
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handleSetModel(ctx, params)
	case "session/set_config_option":
		var params SetConfigOptionParams
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, err
		}
		return s.handleSetConfigOption(ctx, params)
	default:
		return nil, NewRPCError(CodeMethodNotFound, fmt.Sprintf("method not found: %s", method))
	}
}

func (s *Server) handleNotification(method string, rawParams json.RawMessage) {
	if method != "session/cancel" {
		return
	}
	var params CancelParams
	if err := json.Unmarshal(rawParams, &params); err != nil || params.SessionID == "" {
		return
	}
	sess, ok := s.getSession(params.SessionID)
	if !ok {
		return
	}
	sess.cancelActiveRun()
	_ = s.backend.CancelSession(sess.workspaceID, sess.id)
}

func (s *Server) handleInitialize(rawParams json.RawMessage) (InitializeResult, error) {
	var params InitializeParams
	if err := decodeOptionalParams(rawParams, &params); err != nil {
		return InitializeResult{}, err
	}

	s.mu.Lock()
	s.initialized = true
	s.clientCaps = params.ClientCapabilities
	s.mu.Unlock()

	return InitializeResult{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: PromptCapabilities{
				Image:           true,
				Audio:           false,
				EmbeddedContext: true,
			},
			MCPCapabilities: MCPCapabilities{
				HTTP: true,
				SSE:  true,
			},
			SessionCapabilities: &SessionCapabilities{
				List: &SessionListCapabilities{},
			},
		},
		AgentInfo: &ImplementationInfo{
			Name:    "prime",
			Title:   "Prime",
			Version: version.Version,
		},
		AuthMethods: AuthMethods(),
	}, nil
}

func (s *Server) handleSessionNew(ctx context.Context, params NewSessionParams) (NewSessionResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return NewSessionResult{}, err
	}
	wsID, absCwd, err := s.ensureWorkspace(ctx, params.Cwd)
	if err != nil {
		return NewSessionResult{}, err
	}

	se, err := s.backend.CreateSession(ctx, wsID, "ACP Session")
	if err != nil {
		return NewSessionResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	sess := s.upsertSession(se.ID, wsID, absCwd)
	mode, _ := sess.getConfigState()
	return NewSessionResult{
		SessionID:     se.ID,
		Modes:         buildSessionModes(mode),
		ConfigOptions: s.sessionConfigOptions(sess),
	}, nil
}

func (s *Server) handleSessionLoad(ctx context.Context, params LoadSessionParams) (LoadSessionResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return LoadSessionResult{}, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return LoadSessionResult{}, NewRPCError(CodeInvalidParams, "sessionId is required")
	}

	wsID, absCwd, err := s.ensureWorkspace(ctx, params.Cwd)
	if err != nil {
		return LoadSessionResult{}, err
	}

	se, err := s.backend.GetSession(ctx, wsID, params.SessionID)
	if err != nil {
		if isNotFoundError(err) {
			return LoadSessionResult{}, NewRPCError(CodeResourceNotFound, fmt.Sprintf("session not found: %s", params.SessionID))
		}
		return LoadSessionResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	msgs, err := s.backend.ListSessionMessages(ctx, wsID, se.ID)
	if err != nil {
		return LoadSessionResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	sess := s.upsertSession(se.ID, wsID, absCwd)

	if err := ReplayMessages(func(update any) error {
		return s.sendSessionUpdate(se.ID, update)
	}, se.ID, msgs, se.Todos); err != nil {
		return LoadSessionResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	mode, _ := sess.getConfigState()
	return LoadSessionResult{
		Modes:         buildSessionModes(mode),
		ConfigOptions: s.sessionConfigOptions(sess),
	}, nil
}

func (s *Server) handleSessionList(ctx context.Context, params ListSessionsParams) (ListSessionsResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return ListSessionsResult{}, err
	}
	wsID, absCwd, err := s.ensureWorkspace(ctx, params.Cwd)
	if err != nil {
		return ListSessionsResult{}, err
	}

	list, err := s.backend.ListSessions(ctx, wsID)
	if err != nil {
		return ListSessionsResult{}, NewRPCError(CodeInternalError, err.Error())
	}

	summaries := make([]SessionSummary, 0, len(list))
	for _, se := range list {
		if se.ParentSessionID != "" {
			continue
		}
		summary := SessionSummary{
			SessionID: se.ID,
			Cwd:       absCwd,
			Title:     se.Title,
		}
		if se.UpdatedAt > 0 {
			summary.UpdatedAt = time.Unix(se.UpdatedAt, 0).UTC().Format(time.RFC3339)
		}
		summaries = append(summaries, summary)
	}
	return ListSessionsResult{
		Sessions: summaries,
	}, nil
}

func (s *Server) handleSetMode(ctx context.Context, params SetModeParams) (SetModeResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return SetModeResult{}, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return SetModeResult{}, NewRPCError(CodeInvalidParams, "sessionId is required")
	}
	sess, ok := s.getSession(params.SessionID)
	if !ok {
		return SetModeResult{}, NewRPCError(CodeResourceNotFound, fmt.Sprintf("session not found: %s", params.SessionID))
	}
	if _, err := s.ApplyMode(sess, params.ModeID); err != nil {
		return SetModeResult{}, err
	}
	return SetModeResult{}, nil
}

func (s *Server) handleSetModel(ctx context.Context, params SetModelParams) (SetModelResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return SetModelResult{}, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return SetModelResult{}, NewRPCError(CodeInvalidParams, "sessionId is required")
	}
	sess, ok := s.getSession(params.SessionID)
	if !ok {
		return SetModelResult{}, NewRPCError(CodeResourceNotFound, fmt.Sprintf("session not found: %s", params.SessionID))
	}
	options, err := s.ApplyModel(ctx, sess, params.ModelID)
	if err != nil {
		return SetModelResult{}, err
	}
	return SetModelResult{ConfigOptions: options}, nil
}

func (s *Server) handleSetConfigOption(ctx context.Context, params SetConfigOptionParams) (SetConfigOptionResult, error) {
	if err := s.EnsureAuthenticated(ctx); err != nil {
		return SetConfigOptionResult{}, err
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return SetConfigOptionResult{}, NewRPCError(CodeInvalidParams, "sessionId is required")
	}
	sess, ok := s.getSession(params.SessionID)
	if !ok {
		return SetConfigOptionResult{}, NewRPCError(CodeResourceNotFound, fmt.Sprintf("session not found: %s", params.SessionID))
	}

	var options []SessionConfigOption
	var err error
	switch params.ConfigID {
	case ConfigIDMode:
		options, err = s.ApplyMode(sess, params.Value)
	case ConfigIDModel:
		options, err = s.ApplyModel(ctx, sess, params.Value)
	case ConfigIDThoughtLevel:
		options, err = s.ApplyThoughtLevel(ctx, sess, params.Value)
	default:
		return SetConfigOptionResult{}, NewRPCError(CodeInvalidParams, fmt.Sprintf("unknown config option: %s", params.ConfigID))
	}
	if err != nil {
		return SetConfigOptionResult{}, err
	}
	return SetConfigOptionResult{ConfigOptions: options}, nil
}

func (s *Server) ensureWorkspace(ctx context.Context, rawCwd string) (string, string, error) {
	cwd := strings.TrimSpace(rawCwd)
	if cwd == "" {
		cwd = strings.TrimSpace(s.opts.DefaultCwd)
	}
	if cwd == "" {
		return "", "", NewRPCError(CodeInvalidParams, "cwd is required")
	}

	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", NewRPCError(CodeInvalidParams, fmt.Sprintf("invalid cwd: %v", err))
	}
	info, err := os.Stat(absCwd)
	if err != nil || !info.IsDir() {
		return "", "", NewRPCError(CodeInvalidParams, fmt.Sprintf("cwd does not exist or is not a directory: %s", cwd))
	}

	s.mu.Lock()
	if wsID, ok := s.workspaces[absCwd]; ok {
		s.mu.Unlock()
		return wsID, absCwd, nil
	}
	s.mu.Unlock()

	s.wsInitMu.Lock()
	defer s.wsInitMu.Unlock()

	s.mu.Lock()
	if wsID, ok := s.workspaces[absCwd]; ok {
		s.mu.Unlock()
		return wsID, absCwd, nil
	}
	s.mu.Unlock()

	ws, err := s.backend.CreateWorkspace(proto.Workspace{
		Path:     absCwd,
		ClientID: s.clientID,
		YOLO:     s.opts.YOLO,
		Debug:    s.opts.Debug,
		DataDir:  s.opts.DataDir,
		Version:  version.Version,
		Env:      os.Environ(),
	})
	if err != nil {
		return "", "", NewRPCError(CodeInternalError, err.Error())
	}

	if err := s.backend.AttachClient(ws.ID, s.clientID); err != nil {
		return "", "", NewRPCError(CodeInternalError, err.Error())
	}

	s.mu.Lock()
	s.workspaces[absCwd] = ws.ID
	if !slices.Contains(s.attached, ws.ID) {
		s.attached = append(s.attached, ws.ID)
	}
	s.mu.Unlock()

	if err := s.backend.InitAgent(ctx, ws.ID, false); err != nil {
		slog.Warn("Failed to initialize ACP agent", "workspace_id", ws.ID, "error", err)
	}

	return ws.ID, absCwd, nil
}

func decodeParams(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return NewRPCError(CodeInvalidParams, "params are required")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return NewRPCError(CodeInvalidParams, fmt.Sprintf("invalid params: %v", err))
	}
	return nil
}

func decodeOptionalParams(raw json.RawMessage, dst any) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return NewRPCError(CodeInvalidParams, fmt.Sprintf("invalid params: %v", err))
	}
	return nil
}

func isNotFoundError(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "not found") || strings.Contains(lower, "no rows")
}
