package acp

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/internal/session"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type fakeAccount struct {
	mu       sync.Mutex
	signedIn bool
	waiting  bool
	loginErr error
	stateErr string
	onStart  func()
}

func (f *fakeAccount) Restore(_ context.Context) account.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return account.State{SignedIn: f.signedIn}
}

func (f *fakeAccount) StartLogin(_ context.Context) (account.State, error) {
	f.mu.Lock()
	err := f.loginErr
	hook := f.onStart
	if err == nil {
		f.waiting = true
	}
	st := account.State{SignedIn: f.signedIn, Waiting: f.waiting, LoginURL: "https://example.test/login"}
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	return st, err
}

func (f *fakeAccount) State(_ context.Context) account.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return account.State{
		SignedIn: f.signedIn,
		Waiting:  f.waiting,
		Error:    f.stateErr,
	}
}

func (f *fakeAccount) setSignedIn(v bool) {
	f.mu.Lock()
	f.signedIn = v
	f.waiting = false
	f.mu.Unlock()
}

type fakeBackend struct {
	mu sync.Mutex

	createdWorkspaces []proto.Workspace
	attachedClients   map[string]int
	detachedClients   map[string]int
	initInteractive   []bool
	mainAgent         string
	cfg               *config.Config
	agentInfo         proto.AgentInfo

	sessions map[string]session.Session
	messages map[string][]message.Message

	sentMessages []proto.AgentMessage
	cancelled    []string
	grants       []proto.PermissionGrant

	eventBroker *pubsub.Broker[tea.Msg]
	onSend      func(b *fakeBackend, wsID string, msg proto.AgentMessage)
	onGrant     func(req proto.PermissionGrant)
	onCancel    func(sessionID string)
}

func newFakeBackend() *fakeBackend {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("prov-a", config.ProviderConfig{
		ID:   "prov-a",
		Name: "Provider A",
		Models: []catwalk.Model{
			{
				ID:                     "alpha-1",
				Name:                   "Alpha 1",
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
				DefaultMaxTokens:       4096,
			},
			{
				ID:               "beta-2",
				Name:             "Beta 2",
				CanReason:        false,
				DefaultMaxTokens: 4096,
			},
		},
	})

	cfg := &config.Config{
		Providers:     providers,
		SessionModels: make(map[string]config.SelectedModel),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Provider: "prov-a",
				Model:    "alpha-1",
			},
		},
	}

	return &fakeBackend{
		attachedClients: make(map[string]int),
		detachedClients: make(map[string]int),
		mainAgent:       config.AgentGeneral,
		cfg:             cfg,
		agentInfo: proto.AgentInfo{
			IsReady: true,
			Model: catwalk.Model{
				ID:                     "alpha-1",
				Name:                   "Alpha 1",
				CanReason:              true,
				ReasoningLevels:        []string{"low", "medium", "high"},
				DefaultReasoningEffort: "medium",
			},
			ModelCfg: config.SelectedModel{
				Provider: "prov-a",
				Model:    "alpha-1",
			},
		},
		sessions:    make(map[string]session.Session),
		messages:    make(map[string][]message.Message),
		eventBroker: pubsub.NewBroker[tea.Msg](),
	}
}

func (f *fakeBackend) emit(payload tea.Msg) {
	f.eventBroker.Publish(pubsub.UpdatedEvent, payload)
}

func (f *fakeBackend) CreateWorkspace(args proto.Workspace) (proto.Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	args.ID = "ws-1"
	args.Config = f.cfg
	f.createdWorkspaces = append(f.createdWorkspaces, args)
	return args, nil
}

func (f *fakeBackend) AttachClient(workspaceID, clientID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attachedClients[workspaceID+":"+clientID]++
	return nil
}

func (f *fakeBackend) DetachClient(workspaceID, clientID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.detachedClients[workspaceID+":"+clientID]++
}

func (f *fakeBackend) InitAgent(_ context.Context, _ string, interactive bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initInteractive = append(f.initInteractive, interactive)
	return nil
}

func (f *fakeBackend) UpdateAgent(_ context.Context, _ string) error {
	return nil
}

func (f *fakeBackend) SetMainAgent(_ string, agentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mainAgent = agentID
	return nil
}

func (f *fakeBackend) GetWorkspaceConfig(_ string) (*config.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg, nil
}

func (f *fakeBackend) GetAgentInfo(_ string) (proto.AgentInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.agentInfo, nil
}

func (f *fakeBackend) CreateSession(_ context.Context, _ string, title string) (session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := "sess-" + string(rune('1'+len(f.sessions)))
	se := session.Session{
		ID:        id,
		Title:     title,
		UpdatedAt: 1700000000,
	}
	f.sessions[id] = se
	return se, nil
}

func (f *fakeBackend) GetSession(_ context.Context, _ string, sessionID string) (session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	se, ok := f.sessions[sessionID]
	if !ok {
		return session.Session{}, sql.ErrNoRows
	}
	return se, nil
}

func (f *fakeBackend) ListSessions(_ context.Context, _ string) ([]session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]session.Session, 0, len(f.sessions))
	for _, se := range f.sessions {
		out = append(out, se)
	}
	return out, nil
}

func (f *fakeBackend) ListSessionMessages(_ context.Context, _ string, sessionID string) ([]message.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]message.Message(nil), f.messages[sessionID]...), nil
}

func (f *fakeBackend) SendMessage(workspaceID string, msg proto.AgentMessage) error {
	f.mu.Lock()
	f.sentMessages = append(f.sentMessages, msg)
	hook := f.onSend
	f.mu.Unlock()
	if hook != nil {
		go hook(f, workspaceID, msg)
	}
	return nil
}

func (f *fakeBackend) CancelSession(_ string, sessionID string) error {
	f.mu.Lock()
	f.cancelled = append(f.cancelled, sessionID)
	hook := f.onCancel
	f.mu.Unlock()
	if hook != nil {
		hook(sessionID)
	}
	return nil
}

func (f *fakeBackend) GrantPermission(_ string, req proto.PermissionGrant) (bool, error) {
	f.mu.Lock()
	f.grants = append(f.grants, req)
	hook := f.onGrant
	f.mu.Unlock()
	if hook != nil {
		hook(req)
	}
	return true, nil
}

func (f *fakeBackend) UpdateSessionModel(_ string, sessionID string, model *config.SelectedModel) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if model == nil {
		delete(f.cfg.SessionModels, sessionID)
		return nil
	}
	f.cfg.SessionModels[sessionID] = *model
	if m := f.cfg.GetModel(model.Provider, model.Model); m != nil {
		f.agentInfo.Model = *m
	}
	f.agentInfo.ModelCfg = *model
	return nil
}

func (f *fakeBackend) SubscribeEvents(ctx context.Context, _ string) (<-chan pubsub.Event[tea.Msg], error) {
	return f.eventBroker.Subscribe(ctx), nil
}

type acpTestHarness struct {
	t        *testing.T
	backend  *fakeBackend
	account  *fakeAccount
	c2sW     *io.PipeWriter
	s2cR     *io.PipeReader
	reader   *bufio.Reader
	serveErr chan error
	cwd      string
}

func newACPTestHarness(t *testing.T, signedIn bool, yolo bool) *acpTestHarness {
	t.Helper()

	cwd := t.TempDir()
	fb := newFakeBackend()
	fa := &fakeAccount{signedIn: signedIn}

	c2sR, c2sW := io.Pipe()
	s2cR, s2cW := io.Pipe()

	srv := NewServer(fb, fa, c2sR, s2cW, ServerOptions{
		DefaultCwd:        cwd,
		YOLO:              yolo,
		AuthPollInterval:  10 * time.Millisecond,
		CancelWaitTimeout: 150 * time.Millisecond,
	})

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(context.Background())
		_ = s2cW.Close()
	}()

	h := &acpTestHarness{
		t:        t,
		backend:  fb,
		account:  fa,
		c2sW:     c2sW,
		s2cR:     s2cR,
		reader:   bufio.NewReaderSize(s2cR, 64*1024),
		serveErr: serveErr,
		cwd:      cwd,
	}
	t.Cleanup(func() {
		_ = c2sW.Close()
		_ = s2cR.Close()
		select {
		case <-serveErr:
		case <-time.After(2 * time.Second):
		}
	})
	return h
}

func (h *acpTestHarness) send(msg any) {
	h.t.Helper()
	raw, err := json.Marshal(msg)
	require.NoError(h.t, err)
	_, err = h.c2sW.Write(append(raw, '\n'))
	require.NoError(h.t, err)
}

func (h *acpTestHarness) readMessage() RPCMessage {
	h.t.Helper()
	line, err := readNDJSONLine(h.reader, MaxLineBytes)
	require.NoError(h.t, err)
	var msg RPCMessage
	require.NoError(h.t, json.Unmarshal(line, &msg))
	return msg
}

func (h *acpTestHarness) initialize() InitializeResult {
	h.t.Helper()
	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": ProtocolVersion,
		},
	})
	resp := h.readMessage()
	require.Nil(h.t, resp.Error)
	var res InitializeResult
	require.NoError(h.t, json.Unmarshal(resp.Result, &res))
	return res
}

func (h *acpTestHarness) newSession(id int) NewSessionResult {
	h.t.Helper()
	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      id,
		"method":  "session/new",
		"params":  map[string]any{"cwd": h.cwd},
	})
	resp := h.readMessage()
	require.Nil(h.t, resp.Error)
	var res NewSessionResult
	require.NoError(h.t, json.Unmarshal(resp.Result, &res))
	return res
}

func TestServer_InitializeAndRejectUninitialized(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, false)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      10,
		"method":  "session/new",
		"params":  map[string]any{"cwd": h.cwd},
	})
	earlyResp := h.readMessage()
	require.NotNil(t, earlyResp.Error)
	require.Equal(t, CodeInvalidRequest, earlyResp.Error.Code)

	initRes := h.initialize()
	require.Equal(t, ProtocolVersion, initRes.ProtocolVersion)
	require.True(t, initRes.AgentCapabilities.LoadSession)
	require.True(t, initRes.AgentCapabilities.PromptCapabilities.Image)
	require.True(t, initRes.AgentCapabilities.PromptCapabilities.EmbeddedContext)
	require.NotNil(t, initRes.AgentCapabilities.SessionCapabilities)
	require.NotNil(t, initRes.AgentCapabilities.SessionCapabilities.List)
	require.Equal(t, "prime", initRes.AgentInfo.Name)
	require.Len(t, initRes.AuthMethods, 1)
	require.Equal(t, AuthMethodPrimeGoogle, initRes.AuthMethods[0].ID)
}

func TestServer_AuthRequiredAndAuthenticate(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, false, false)
	h.initialize()

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      2,
		"method":  "session/new",
		"params":  map[string]any{"cwd": h.cwd},
	})
	unauthResp := h.readMessage()
	require.NotNil(t, unauthResp.Error)
	require.Equal(t, CodeAuthRequired, unauthResp.Error.Code)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      3,
		"method":  "authenticate",
		"params":  map[string]any{"methodId": "unknown"},
	})
	badMethodResp := h.readMessage()
	require.NotNil(t, badMethodResp.Error)
	require.Equal(t, CodeInvalidParams, badMethodResp.Error.Code)

	h.account.onStart = func() {
		go func() {
			time.Sleep(25 * time.Millisecond)
			h.account.setSignedIn(true)
		}()
	}

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      4,
		"method":  "authenticate",
		"params":  map[string]any{"methodId": AuthMethodPrimeGoogle},
	})
	authResp := h.readMessage()
	require.Nil(t, authResp.Error)

	newRes := h.newSession(5)
	require.NotEmpty(t, newRes.SessionID)
}

func TestServer_SessionNewLoadAndList(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, false)
	h.initialize()

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      2,
		"method":  "session/new",
		"params":  map[string]any{"cwd": h.cwd + "/does-not-exist"},
	})
	badCwdResp := h.readMessage()
	require.NotNil(t, badCwdResp.Error)
	require.Equal(t, CodeInvalidParams, badCwdResp.Error.Code)

	h.backend.sessions["hist-1"] = session.Session{
		ID:        "hist-1",
		Title:     "History Session",
		UpdatedAt: 1700000123,
		Todos: []session.Todo{
			{Content: "Finish task", Status: session.TodoStatusInProgress},
		},
	}
	h.backend.sessions["child-1"] = session.Session{
		ID:              "child-1",
		ParentSessionID: "hist-1",
		Title:           "Subtask",
	}
	h.backend.messages["hist-1"] = []message.Message{
		{
			ID:        "u-1",
			SessionID: "hist-1",
			Role:      message.User,
			Parts:     []message.ContentPart{message.TextContent{Text: "Hello"}},
		},
		{
			ID:        "a-1",
			SessionID: "hist-1",
			Role:      message.Assistant,
			Parts:     []message.ContentPart{message.TextContent{Text: "Hi there"}},
		},
	}

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      3,
		"method":  "session/load",
		"params":  map[string]any{"sessionId": "missing", "cwd": h.cwd},
	})
	missingResp := h.readMessage()
	require.NotNil(t, missingResp.Error)
	require.Equal(t, CodeResourceNotFound, missingResp.Error.Code)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      4,
		"method":  "session/load",
		"params":  map[string]any{"sessionId": "hist-1", "cwd": h.cwd},
	})

	var replayKinds []string
	var loadResp RPCMessage
	for {
		msg := h.readMessage()
		if msg.Method == "session/update" {
			var up struct {
				SessionID string `json:"sessionId"`
				Update    struct {
					SessionUpdate string `json:"sessionUpdate"`
				} `json:"update"`
			}
			require.NoError(t, json.Unmarshal(msg.Params, &up))
			require.Equal(t, "hist-1", up.SessionID)
			replayKinds = append(replayKinds, up.Update.SessionUpdate)
			continue
		}
		loadResp = msg
		break
	}

	require.Nil(t, loadResp.Error)
	require.Equal(t, []string{"user_message_chunk", "agent_message_chunk", "plan"}, replayKinds)

	var loadRes LoadSessionResult
	require.NoError(t, json.Unmarshal(loadResp.Result, &loadRes))
	require.NotNil(t, loadRes.Modes)
	require.Equal(t, ModeCoder, loadRes.Modes.CurrentModeID)
	require.NotEmpty(t, loadRes.ConfigOptions)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      5,
		"method":  "session/list",
		"params":  map[string]any{"cwd": h.cwd},
	})
	listResp := h.readMessage()
	require.Nil(t, listResp.Error)
	var listRes ListSessionsResult
	require.NoError(t, json.Unmarshal(listResp.Result, &listRes))
	require.Len(t, listRes.Sessions, 1)
	require.Equal(t, "hist-1", listRes.Sessions[0].SessionID)
	require.Equal(t, "History Session", listRes.Sessions[0].Title)

	require.NoError(t, h.c2sW.Close())
	require.NoError(t, <-h.serveErr)

	h.backend.mu.Lock()
	require.Len(t, h.backend.initInteractive, 1)
	require.False(t, h.backend.initInteractive[0])
	require.Len(t, h.backend.attachedClients, 1)
	require.Len(t, h.backend.detachedClients, 1)
	h.backend.mu.Unlock()
}

func TestServer_PromptStreamingBlocksAndStopReasons(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, false)
	h.initialize()
	sess := h.newSession(2)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      3,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": sess.SessionID,
			"prompt":    []ContentBlock{},
		},
	})
	emptyResp := h.readMessage()
	require.NotNil(t, emptyResp.Error)
	require.Equal(t, CodeInvalidParams, emptyResp.Error.Code)

	h.backend.onSend = func(b *fakeBackend, _ string, msg proto.AgentMessage) {
		b.emit(pubsub.Event[message.Message]{
			Type: pubsub.UpdatedEvent,
			Payload: message.Message{
				ID:        "a-turn-1",
				SessionID: msg.SessionID,
				Role:      message.Assistant,
				Parts:     []message.ContentPart{message.TextContent{Text: "Part 1"}},
			},
		})
		b.emit(pubsub.Event[message.Message]{
			Type: pubsub.UpdatedEvent,
			Payload: message.Message{
				ID:        "a-turn-1",
				SessionID: msg.SessionID,
				Role:      message.Assistant,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Part 1 and Part 2"},
					message.Finish{Reason: message.FinishReasonEndTurn},
				},
			},
		})
		b.emit(pubsub.Event[notify.RunComplete]{
			Type: pubsub.UpdatedEvent,
			Payload: notify.RunComplete{
				SessionID: msg.SessionID,
				RunID:     msg.RunID,
				MessageID: "a-turn-1",
				Text:      "Part 1 and Part 2!",
			},
		})
	}

	imgPayload := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47})
	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      4,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": sess.SessionID,
			"prompt": []ContentBlock{
				{Type: "text", Text: "Analyze image"},
				{Type: "image", Data: imgPayload, MimeType: "image/png"},
				{
					Type: "resource",
					Resource: &EmbeddedResource{
						URI:  "file:///tmp/demo.txt",
						Text: "file body",
					},
				},
			},
		},
	})

	var chunks []string
	var promptResp RPCMessage
	for {
		msg := h.readMessage()
		if msg.Method == "session/update" {
			var up struct {
				Update AgentMessageChunkUpdate `json:"update"`
			}
			require.NoError(t, json.Unmarshal(msg.Params, &up))
			chunks = append(chunks, up.Update.Content.Text)
			continue
		}
		promptResp = msg
		break
	}

	require.Nil(t, promptResp.Error)
	var pRes PromptResult
	require.NoError(t, json.Unmarshal(promptResp.Result, &pRes))
	require.Equal(t, StopReasonEndTurn, pRes.StopReason)
	require.Equal(t, []string{"Part 1", " and Part 2", "!"}, chunks)

	h.backend.mu.Lock()
	require.Len(t, h.backend.sentMessages, 1)
	require.Contains(t, h.backend.sentMessages[0].Prompt, "Analyze image")
	require.Contains(t, h.backend.sentMessages[0].Prompt, "file body")
	require.Len(t, h.backend.sentMessages[0].Attachments, 1)
	require.Equal(t, "image/png", h.backend.sentMessages[0].Attachments[0].MimeType)
	h.backend.mu.Unlock()

	for idx, tc := range []struct {
		finishReason message.FinishReason
		wantStop     string
	}{
		{message.FinishReasonMaxTokens, StopReasonMaxTokens},
		{message.FinishReasonContentFilter, StopReasonRefusal},
	} {
		h.backend.onSend = func(b *fakeBackend, _ string, msg proto.AgentMessage) {
			b.emit(pubsub.Event[message.Message]{
				Type: pubsub.UpdatedEvent,
				Payload: message.Message{
					ID:        "a-stop-" + string(rune('1'+idx)),
					SessionID: msg.SessionID,
					Role:      message.Assistant,
					Parts: []message.ContentPart{
						message.TextContent{Text: "stopped"},
						message.Finish{Reason: tc.finishReason},
					},
				},
			})
			b.emit(pubsub.Event[notify.RunComplete]{
				Type: pubsub.UpdatedEvent,
				Payload: notify.RunComplete{
					SessionID: msg.SessionID,
					RunID:     msg.RunID,
					MessageID: "a-stop-" + string(rune('1'+idx)),
					Text:      "stopped",
				},
			})
		}

		h.send(map[string]any{
			"jsonrpc": JSONRPCVersion,
			"id":      10 + idx,
			"method":  "session/prompt",
			"params": map[string]any{
				"sessionId": sess.SessionID,
				"prompt":    []ContentBlock{{Type: "text", Text: "go"}},
			},
		})

		var resp RPCMessage
		for {
			msg := h.readMessage()
			if msg.Method == "session/update" {
				continue
			}
			resp = msg
			break
		}
		require.Nil(t, resp.Error)
		var res PromptResult
		require.NoError(t, json.Unmarshal(resp.Result, &res))
		require.Equal(t, tc.wantStop, res.StopReason)
	}
}

func TestServer_PermissionOutcomes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		outcome    PermissionOutcome
		wantAction proto.PermissionAction
	}{
		{
			name:       "allow_once",
			outcome:    PermissionOutcome{Outcome: PermissionOutcomeSelected, OptionID: PermissionOptionAllowOnce},
			wantAction: proto.PermissionAllow,
		},
		{
			name:       "allow_always",
			outcome:    PermissionOutcome{Outcome: PermissionOutcomeSelected, OptionID: PermissionOptionAllowAlways},
			wantAction: proto.PermissionAllowForSession,
		},
		{
			name:       "reject_once",
			outcome:    PermissionOutcome{Outcome: PermissionOutcomeSelected, OptionID: PermissionOptionRejectOnce},
			wantAction: proto.PermissionDeny,
		},
		{
			name:       "cancelled",
			outcome:    PermissionOutcome{Outcome: PermissionOutcomeCancelled},
			wantAction: proto.PermissionDeny,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newACPTestHarness(t, true, false)
			h.initialize()
			sess := h.newSession(2)

			grantedCh := make(chan proto.PermissionGrant, 1)
			h.backend.onGrant = func(req proto.PermissionGrant) {
				grantedCh <- req
			}
			h.backend.onSend = func(b *fakeBackend, _ string, msg proto.AgentMessage) {
				b.emit(pubsub.Event[permission.PermissionRequest]{
					Type: pubsub.CreatedEvent,
					Payload: permission.PermissionRequest{
						ID:          "perm-1",
						SessionID:   msg.SessionID,
						ToolCallID:  "tc-1",
						ToolName:    "bash",
						Description: "Run command",
						Action:      "execute",
						Path:        h.cwd,
					},
				})
				<-grantedCh
				b.emit(pubsub.Event[notify.RunComplete]{
					Type: pubsub.UpdatedEvent,
					Payload: notify.RunComplete{
						SessionID: msg.SessionID,
						RunID:     msg.RunID,
					},
				})
			}

			h.send(map[string]any{
				"jsonrpc": JSONRPCVersion,
				"id":      3,
				"method":  "session/prompt",
				"params": map[string]any{
					"sessionId": sess.SessionID,
					"prompt":    []ContentBlock{{Type: "text", Text: "run test"}},
				},
			})

			permReq := h.readMessage()
			require.Equal(t, "session/request_permission", permReq.Method)
			require.NotNil(t, permReq.ID)

			var permParams RequestPermissionParams
			require.NoError(t, json.Unmarshal(permReq.Params, &permParams))
			require.Equal(t, sess.SessionID, permParams.SessionID)
			require.Equal(t, "tc-1", permParams.ToolCall.ToolCallID)
			require.Len(t, permParams.Options, 3)

			h.send(map[string]any{
				"jsonrpc": JSONRPCVersion,
				"id":      permReq.ID,
				"result":  RequestPermissionResult{Outcome: tc.outcome},
			})

			promptResp := h.readMessage()
			require.Nil(t, promptResp.Error)

			h.backend.mu.Lock()
			require.Len(t, h.backend.grants, 1)
			require.Equal(t, tc.wantAction, h.backend.grants[0].Action)
			h.backend.mu.Unlock()
		})
	}
}

func TestServer_CancelPromptAndPendingPermission(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, false)
	h.initialize()
	sess := h.newSession(2)

	grantDone := make(chan proto.PermissionAction, 1)
	h.backend.onGrant = func(req proto.PermissionGrant) {
		grantDone <- req.Action
	}

	h.backend.onSend = func(b *fakeBackend, _ string, msg proto.AgentMessage) {
		b.emit(pubsub.Event[permission.PermissionRequest]{
			Type: pubsub.CreatedEvent,
			Payload: permission.PermissionRequest{
				ID:          "perm-cancel-1",
				SessionID:   msg.SessionID,
				ToolCallID:  "tc-cancel",
				ToolName:    "bash",
				Description: "Long command",
			},
		})
	}

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      3,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": sess.SessionID,
			"prompt":    []ContentBlock{{Type: "text", Text: "run"}},
		},
	})

	permReq := h.readMessage()
	require.Equal(t, "session/request_permission", permReq.Method)
	require.NotNil(t, permReq.ID)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  "session/cancel",
		"params":  CancelParams{SessionID: sess.SessionID},
	})

	var sawCancelReq bool
	var promptResp RPCMessage
	for {
		msg := h.readMessage()
		if msg.Method == "$/cancel_request" {
			var cp CancelRequestParams
			require.NoError(t, json.Unmarshal(msg.Params, &cp))
			require.Equal(t, permReq.ID.Key(), cp.RequestID.Key())
			sawCancelReq = true
			continue
		}
		if msg.ID != nil && msg.ID.String() == "3" {
			promptResp = msg
			break
		}
	}

	require.True(t, sawCancelReq)
	require.Nil(t, promptResp.Error)
	var pRes PromptResult
	require.NoError(t, json.Unmarshal(promptResp.Result, &pRes))
	require.Equal(t, StopReasonCancelled, pRes.StopReason)

	select {
	case act := <-grantDone:
		require.Equal(t, proto.PermissionDeny, act)
	case <-time.After(2 * time.Second):
		t.Fatal("expected pending permission to be denied on cancel")
	}
}

func TestServer_ConfigOptionsModeModelThoughtLevel(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, false)
	h.initialize()
	sess := h.newSession(2)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      3,
		"method":  "session/set_mode",
		"params":  SetModeParams{SessionID: sess.SessionID, ModeID: ModePlan},
	})

	var sawModeUpdate bool
	var sawConfigUpdate bool
	var setModeResp RPCMessage
	for {
		msg := h.readMessage()
		if msg.Method == "session/update" {
			var up struct {
				Update struct {
					SessionUpdate string `json:"sessionUpdate"`
					CurrentModeID string `json:"currentModeId"`
				} `json:"update"`
			}
			require.NoError(t, json.Unmarshal(msg.Params, &up))
			if up.Update.SessionUpdate == "current_mode_update" {
				require.Equal(t, ModePlan, up.Update.CurrentModeID)
				sawModeUpdate = true
			}
			if up.Update.SessionUpdate == "config_option_update" {
				sawConfigUpdate = true
			}
			continue
		}
		setModeResp = msg
		break
	}
	require.Nil(t, setModeResp.Error)
	require.True(t, sawModeUpdate)
	require.True(t, sawConfigUpdate)

	h.backend.mu.Lock()
	require.Equal(t, config.AgentPlan, h.backend.mainAgent)
	h.backend.mu.Unlock()

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      4,
		"method":  "session/set_config_option",
		"params": SetConfigOptionParams{
			SessionID: sess.SessionID,
			ConfigID:  ConfigIDThoughtLevel,
			Value:     "high",
		},
	})
	var thoughtResp RPCMessage
	for {
		msg := h.readMessage()
		if msg.Method == "session/update" {
			continue
		}
		thoughtResp = msg
		break
	}
	require.Nil(t, thoughtResp.Error)
	var thoughtRes SetConfigOptionResult
	require.NoError(t, json.Unmarshal(thoughtResp.Result, &thoughtRes))
	foundThought := false
	for _, opt := range thoughtRes.ConfigOptions {
		if opt.ID == ConfigIDThoughtLevel {
			foundThought = true
			require.Equal(t, "high", opt.CurrentValue)
		}
	}
	require.True(t, foundThought)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      5,
		"method":  "session/set_model",
		"params":  SetModelParams{SessionID: sess.SessionID, ModelID: "prov-a/beta-2"},
	})
	var modelResp RPCMessage
	for {
		msg := h.readMessage()
		if msg.Method == "session/update" {
			continue
		}
		modelResp = msg
		break
	}
	require.Nil(t, modelResp.Error)
	var modelRes SetModelResult
	require.NoError(t, json.Unmarshal(modelResp.Result, &modelRes))
	for _, opt := range modelRes.ConfigOptions {
		if opt.ID == ConfigIDModel {
			require.Equal(t, "prov-a/beta-2", opt.CurrentValue)
		}
		require.NotEqual(t, ConfigIDThoughtLevel, opt.ID)
	}

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      6,
		"method":  "session/set_config_option",
		"params": SetConfigOptionParams{
			SessionID: sess.SessionID,
			ConfigID:  "unknown_option",
			Value:     "x",
		},
	})
	unknownOptResp := h.readMessage()
	require.NotNil(t, unknownOptResp.Error)
	require.Equal(t, CodeInvalidParams, unknownOptResp.Error.Code)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      7,
		"method":  "session/set_config_option",
		"params": SetConfigOptionParams{
			SessionID: "no-such-session",
			ConfigID:  ConfigIDMode,
			Value:     ModeCoder,
		},
	})
	unknownSessResp := h.readMessage()
	require.NotNil(t, unknownSessResp.Error)
	require.Equal(t, CodeResourceNotFound, unknownSessResp.Error.Code)

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      8,
		"method":  "session/set_config_option",
		"params": SetConfigOptionParams{
			SessionID: sess.SessionID,
			ConfigID:  ConfigIDModel,
			Value:     "prov-a/missing-model",
		},
	})
	badModelResp := h.readMessage()
	require.NotNil(t, badModelResp.Error)
	require.Equal(t, CodeInvalidParams, badModelResp.Error.Code)
}

func TestServer_YOLOModePassedToWorkspace(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, true)
	h.initialize()
	_ = h.newSession(2)

	h.backend.mu.Lock()
	require.Len(t, h.backend.createdWorkspaces, 1)
	require.True(t, h.backend.createdWorkspaces[0].YOLO)
	h.backend.mu.Unlock()
}

func TestServer_PromptErrorReturnsRPCError(t *testing.T) {
	t.Parallel()

	h := newACPTestHarness(t, true, false)
	h.initialize()
	sess := h.newSession(2)

	h.backend.onSend = func(b *fakeBackend, _ string, msg proto.AgentMessage) {
		b.emit(pubsub.Event[notify.RunComplete]{
			Type: pubsub.UpdatedEvent,
			Payload: notify.RunComplete{
				SessionID: msg.SessionID,
				RunID:     msg.RunID,
				Error:     errors.New("upstream failure").Error(),
			},
		})
	}

	h.send(map[string]any{
		"jsonrpc": JSONRPCVersion,
		"id":      3,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": sess.SessionID,
			"prompt":    []ContentBlock{{Type: "text", Text: "hello"}},
		},
	})

	resp := h.readMessage()
	require.NotNil(t, resp.Error)
	require.Equal(t, CodeInternalError, resp.Error.Code)
	require.Contains(t, resp.Error.Message, "upstream failure")
}
