package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/customagents"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type mockCustomAgentCatalog struct {
	models []customagents.ModelOption
}

func (m *mockCustomAgentCatalog) ListAvailableModels(_ context.Context) ([]customagents.ModelOption, error) {
	return m.models, nil
}

type mockCustomAgentExecutor struct {
	mu      sync.Mutex
	handler func(ctx context.Context, req customagents.ExecuteRequest) (customagents.ExecuteResult, error)
}

func (m *mockCustomAgentExecutor) Execute(ctx context.Context, req customagents.ExecuteRequest) (customagents.ExecuteResult, error) {
	m.mu.Lock()
	h := m.handler
	m.mu.Unlock()
	if h != nil {
		return h(ctx, req)
	}
	if req.OnStep != nil {
		req.OnStep(customagents.RunStep{
			Kind:   "llm_step",
			Title:  "Done",
			Detail: "ok",
			Status: "ok",
		})
	}
	return customagents.ExecuteResult{OutputText: "executed: " + req.Prompt}, nil
}

func newTestCustomAgentsHTTPServer(t *testing.T, exec *mockCustomAgentExecutor) (*httptest.Server, *customagents.Service) {
	t.Helper()
	store, err := customagents.OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = store.Close()
	})

	broker := pubsub.NewBroker[customagents.Event]()
	t.Cleanup(broker.Shutdown)

	catalog := &mockCustomAgentCatalog{
		models: []customagents.ModelOption{
			{
				ProviderID:           "openai",
				ProviderName:         "OpenAI",
				ModelID:              "gpt-5-mini",
				Name:                 "GPT-5 Mini",
				Tier:                 customagents.TierFast,
				CanReason:            true,
				ReasoningModes:       []string{"off", "low", "medium", "high"},
				DefaultReasoningMode: "low",
				ContextWindow:        128000,
				CostPer1MIn:          0.25,
				CostPer1MOut:         2.0,
			},
		},
	}

	svc := customagents.NewService(store, exec, catalog, &customAgentsEventSink{broker: broker})
	t.Cleanup(svc.Stop)

	c := &controllerV1{
		customAgents:      svc,
		customAgentEvents: broker,
	}
	mux := http.NewServeMux()
	for _, ep := range c.customAgentEndpoints() {
		mux.HandleFunc(ep.Method()+" "+ep.Path(), ep.Handler())
	}

	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	return hs, svc
}

func TestCustomAgentsEndpointsCRUDAndModels(t *testing.T) {
	t.Parallel()
	exec := &mockCustomAgentExecutor{}
	hs, _ := newTestCustomAgentsHTTPServer(t, exec)
	client := hs.Client()

	resp, err := client.Get(hs.URL + "/v1/custom-agents")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listBody struct {
		Agents []customagents.CustomAgent `json:"agents"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listBody))
	require.Empty(t, listBody.Agents)

	respModels, err := client.Get(hs.URL + "/v1/custom-agents/models")
	require.NoError(t, err)
	defer respModels.Body.Close()
	require.Equal(t, http.StatusOK, respModels.StatusCode)
	var modelsBody struct {
		Models []customagents.ModelOption `json:"models"`
	}
	require.NoError(t, json.NewDecoder(respModels.Body).Decode(&modelsBody))
	require.Len(t, modelsBody.Models, 1)
	require.Equal(t, "gpt-5-mini", modelsBody.Models[0].ModelID)

	createPayload := map[string]any{
		"name":          "Watcher",
		"slug":          "watcher",
		"description":   "Watches a target file",
		"role":          customagents.RoleWorker,
		"system_prompt": "Inspect changes.",
		"model_mode":    customagents.ModelModeAuto,
	}
	rawCreate, err := json.Marshal(createPayload)
	require.NoError(t, err)
	respCreate, err := client.Post(hs.URL+"/v1/custom-agents", "application/json", bytes.NewReader(rawCreate))
	require.NoError(t, err)
	defer respCreate.Body.Close()
	require.Equal(t, http.StatusCreated, respCreate.StatusCode)
	var created customagents.CustomAgent
	require.NoError(t, json.NewDecoder(respCreate.Body).Decode(&created))
	require.NotEmpty(t, created.ID)
	require.Equal(t, "watcher", created.Slug)
	require.True(t, created.Enabled)

	respGet, err := client.Get(hs.URL + "/v1/custom-agents/" + created.ID)
	require.NoError(t, err)
	defer respGet.Body.Close()
	require.Equal(t, http.StatusOK, respGet.StatusCode)
	var fetched customagents.CustomAgent
	require.NoError(t, json.NewDecoder(respGet.Body).Decode(&fetched))
	require.Equal(t, created.ID, fetched.ID)

	patchRaw := []byte(`{"description":"Updated watcher","enabled":false}`)
	patchReq, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, hs.URL+"/v1/custom-agents/"+created.ID, bytes.NewReader(patchRaw))
	require.NoError(t, err)
	patchReq.Header.Set("Content-Type", "application/json")
	respPatch, err := client.Do(patchReq)
	require.NoError(t, err)
	defer respPatch.Body.Close()
	require.Equal(t, http.StatusOK, respPatch.StatusCode)
	var updated customagents.CustomAgent
	require.NoError(t, json.NewDecoder(respPatch.Body).Decode(&updated))
	require.Equal(t, "Updated watcher", updated.Description)
	require.False(t, updated.Enabled)
	require.Equal(t, customagents.StatusPaused, updated.Status)
	require.Equal(t, "Inspect changes.", updated.SystemPrompt)

	delReq, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, hs.URL+"/v1/custom-agents/"+created.ID, nil)
	require.NoError(t, err)
	respDel, err := client.Do(delReq)
	require.NoError(t, err)
	defer respDel.Body.Close()
	require.Equal(t, http.StatusOK, respDel.StatusCode)

	respMissing, err := client.Get(hs.URL + "/v1/custom-agents/" + created.ID)
	require.NoError(t, err)
	defer respMissing.Body.Close()
	require.Equal(t, http.StatusNotFound, respMissing.StatusCode)
}

func TestCustomAgentsEndpointsRunProbeCancelBlackboardAndSSE(t *testing.T) {
	t.Parallel()
	startedCh := make(chan struct{}, 1)
	releaseCh := make(chan struct{})
	exec := &mockCustomAgentExecutor{
		handler: func(ctx context.Context, req customagents.ExecuteRequest) (customagents.ExecuteResult, error) {
			if req.Prompt == "block" {
				select {
				case startedCh <- struct{}{}:
				default:
				}
				select {
				case <-ctx.Done():
					return customagents.ExecuteResult{}, ctx.Err()
				case <-releaseCh:
					return customagents.ExecuteResult{OutputText: "released"}, nil
				}
			}
			return customagents.ExecuteResult{OutputText: "sync-ok: " + req.Prompt}, nil
		},
	}
	hs, svc := newTestCustomAgentsHTTPServer(t, exec)
	client := hs.Client()

	monFile := filepath.Join(t.TempDir(), "watched.txt")
	require.NoError(t, os.WriteFile(monFile, []byte("v1"), 0o600))

	agentObj, err := svc.Store().CreateAgent(t.Context(), customagents.CustomAgent{
		Slug:          "runner",
		Name:          "Runner",
		Role:          customagents.RoleWorker,
		SystemPrompt:  "Run tasks.",
		ModelMode:     customagents.ModelModeAuto,
		Enabled:       true,
		TriggerType:   customagents.TriggerMonitor,
		MonitorKind:   customagents.MonitorFile,
		MonitorTarget: monFile,
	})
	require.NoError(t, err)

	sseCtx, sseCancel := context.WithCancel(t.Context())
	defer sseCancel()
	sseReq, err := http.NewRequestWithContext(sseCtx, http.MethodGet, hs.URL+"/v1/custom-agents/events", nil)
	require.NoError(t, err)
	sseResp, err := client.Do(sseReq)
	require.NoError(t, err)
	defer sseResp.Body.Close()
	require.Equal(t, http.StatusOK, sseResp.StatusCode)
	require.Contains(t, sseResp.Header.Get("Content-Type"), "text/event-stream")

	sseEvents := make(chan customagents.Event, 16)
	sseErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(sseResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				var ev customagents.Event
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev) == nil {
					sseEvents <- ev
				}
			}
		}
		sseErrCh <- scanner.Err()
	}()

	probeResp, err := client.Post(hs.URL+"/v1/custom-agents/"+agentObj.ID+"/probe", "application/json", nil)
	require.NoError(t, err)
	defer probeResp.Body.Close()
	require.Equal(t, http.StatusOK, probeResp.StatusCode)
	var probeResult customagents.ProbeResult
	require.NoError(t, json.NewDecoder(probeResp.Body).Decode(&probeResult))
	require.True(t, probeResult.Changed)
	require.Empty(t, probeResult.Error)

	syncRunBody := []byte(`{"prompt":"hello","chain_id":"chain-1","force":true,"sync":true}`)
	syncResp, err := client.Post(hs.URL+"/v1/custom-agents/"+agentObj.ID+"/run", "application/json", bytes.NewReader(syncRunBody))
	require.NoError(t, err)
	defer syncResp.Body.Close()
	require.Equal(t, http.StatusAccepted, syncResp.StatusCode)
	var syncRun customagents.CustomAgentRun
	require.NoError(t, json.NewDecoder(syncResp.Body).Decode(&syncRun))
	require.Equal(t, customagents.RunPassed, syncRun.Status)
	require.Equal(t, "sync-ok: hello", syncRun.OutputText)

	deadline := time.After(3 * time.Second)
	sawPassed := false
	for !sawPassed {
		select {
		case ev := <-sseEvents:
			if ev.Type == "run.passed" && ev.RunID == syncRun.ID {
				sawPassed = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for SSE run.passed event")
		}
	}
	sseCancel()
	_ = sseResp.Body.Close()
	scanErr := <-sseErrCh
	if scanErr != nil && !errors.Is(scanErr, context.Canceled) {
		require.NoError(t, scanErr)
	}

	asyncRunBody := []byte(`{"prompt":"block","force":true,"sync":false}`)
	asyncResp, err := client.Post(hs.URL+"/v1/custom-agents/"+agentObj.ID+"/run", "application/json", bytes.NewReader(asyncRunBody))
	require.NoError(t, err)
	defer asyncResp.Body.Close()
	require.Equal(t, http.StatusAccepted, asyncResp.StatusCode)
	var asyncRun customagents.CustomAgentRun
	require.NoError(t, json.NewDecoder(asyncResp.Body).Decode(&asyncRun))

	select {
	case <-startedCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async run to start")
	}

	cancelResp, err := client.Post(hs.URL+"/v1/custom-agents/"+agentObj.ID+"/runs/"+asyncRun.ID+"/cancel", "application/json", nil)
	require.NoError(t, err)
	defer cancelResp.Body.Close()
	require.Equal(t, http.StatusOK, cancelResp.StatusCode)
	close(releaseCh)

	require.Eventually(t, func() bool {
		r, getErr := svc.Store().GetRun(t.Context(), asyncRun.ID)
		return getErr == nil && r.Status == customagents.RunCancelled
	}, 3*time.Second, 20*time.Millisecond)

	runsResp, err := client.Get(hs.URL + "/v1/custom-agents/" + agentObj.ID + "/runs")
	require.NoError(t, err)
	defer runsResp.Body.Close()
	require.Equal(t, http.StatusOK, runsResp.StatusCode)
	var agentRunsBody struct {
		Runs []customagents.CustomAgentRun `json:"runs"`
	}
	require.NoError(t, json.NewDecoder(runsResp.Body).Decode(&agentRunsBody))
	require.Len(t, agentRunsBody.Runs, 2)

	globalRunsResp, err := client.Get(hs.URL + "/v1/custom-agents/runs?agent_id=" + agentObj.ID + "&limit=1")
	require.NoError(t, err)
	defer globalRunsResp.Body.Close()
	require.Equal(t, http.StatusOK, globalRunsResp.StatusCode)
	var globalRunsBody struct {
		Runs []customagents.CustomAgentRun `json:"runs"`
	}
	require.NoError(t, json.NewDecoder(globalRunsResp.Body).Decode(&globalRunsBody))
	require.Len(t, globalRunsBody.Runs, 1)

	bbPost := []byte(`{"chain_id":"chain-1","topic":"handoff","payload":"ready"}`)
	bbPostResp, err := client.Post(hs.URL+"/v1/custom-agents/blackboard", "application/json", bytes.NewReader(bbPost))
	require.NoError(t, err)
	defer bbPostResp.Body.Close()
	require.Equal(t, http.StatusCreated, bbPostResp.StatusCode)

	bbGetResp, err := client.Get(hs.URL + "/v1/custom-agents/blackboard?chain_id=chain-1")
	require.NoError(t, err)
	defer bbGetResp.Body.Close()
	require.Equal(t, http.StatusOK, bbGetResp.StatusCode)
	var bbBody struct {
		Entries []customagents.BlackboardEntry `json:"entries"`
	}
	require.NoError(t, json.NewDecoder(bbGetResp.Body).Decode(&bbBody))
	require.Len(t, bbBody.Entries, 1)
	require.Equal(t, "handoff", bbBody.Entries[0].Topic)
	require.Equal(t, "ready", bbBody.Entries[0].Payload)
}

func TestCustomAgentsUnavailableServiceReturns503(t *testing.T) {
	t.Parallel()
	c := &controllerV1{}
	mux := http.NewServeMux()
	for _, ep := range c.customAgentEndpoints() {
		mux.HandleFunc(ep.Method()+" "+ep.Path(), ep.Handler())
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/custom-agents", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
