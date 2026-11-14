package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/env"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type mockPrimeAIClient struct {
	provider catwalk.Provider
	err      error
}

func (m *mockPrimeAIClient) Get(context.Context) (catwalk.Provider, error) {
	return m.provider, m.err
}

func TestPrimeAISync_FallbackAndCache(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "primeai.json")
	syncer := &primeaiSync{}
	syncer.Init(&mockPrimeAIClient{
		provider: catwalk.Provider{
			ID:   primeai.Name,
			Name: primeai.DisplayName,
			Type: primeai.Name,
			Models: []catwalk.Model{
				{ID: primeai.AutomaticModelID, Name: primeai.AutomaticModelName},
				{ID: "gpt-5", Name: "GPT-5"},
			},
		},
	}, path, true)

	p, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, catwalk.InferenceProvider(primeai.Name), p.ID)
	require.Len(t, p.Models, 2)

	errSyncer := &primeaiSync{}
	errSyncer.Init(&mockPrimeAIClient{err: context.DeadlineExceeded}, path, true)
	cached, err := errSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, cached.Models, 2)

	noCacheSyncer := &primeaiSync{}
	noCacheSyncer.Init(&mockPrimeAIClient{err: context.DeadlineExceeded}, filepath.Join(t.TempDir(), "missing.json"), true)
	emb, err := noCacheSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, catwalk.InferenceProvider(primeai.Name), emb.ID)
	require.NotEmpty(t, emb.Models)
	require.Equal(t, primeai.AutomaticModelID, emb.Models[0].ID)
}

func TestProviders_PrependsPrimeAIWhenSignedIn(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("PRIME_GLOBAL_DATA", tmpDir)

	resetProviderState()
	defer resetProviderState()

	origHasToken := hasPrimeAccountToken
	hasPrimeAccountToken = func() bool { return true }
	defer func() { hasPrimeAccountToken = origHasToken }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/prime/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_cfg",
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/prime/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"id": "claude-sonnet", "name": "Claude Sonnet", "routeCount": 1, "kind": "text"},
				},
			})
		}
	}))
	defer srv.Close()
	t.Setenv("PRIMEAI_URL", srv.URL)

	catwalkSyncer.Init(&mockCatwalkClient{
		providers: []catwalk.Provider{{Name: "Provider1", ID: "p1"}},
	}, filepath.Join(tmpDir, "providers.json"), true)
	hyperSyncer.Init(&mockHyperClient{
		provider: catwalk.Provider{
			Name:   "Hyper",
			ID:     "hyper",
			Models: []catwalk.Model{{ID: "h1", Name: "Hyper 1"}},
		},
	}, filepath.Join(tmpDir, "hyper.json"), true)
	primeaiSyncer.Init(realPrimeAIClient{
		accountToken: func() string { return "acct_cfg" },
	}, filepath.Join(tmpDir, "primeai.json"), true)

	_, _ = catwalkSyncer.Get(t.Context())
	_, _ = hyperSyncer.Get(t.Context())
	_, _ = primeaiSyncer.Get(t.Context())

	providers, err := Providers(&Config{Options: &Options{}})
	require.NoError(t, err)
	require.Len(t, providers, 3)
	require.Equal(t, catwalk.InferenceProvider(primeai.Name), providers[0].ID)
	require.Equal(t, catwalk.InferenceProvider("hyper"), providers[1].ID)

	cfg := &Config{}
	cfg.setDefaults(tmpDir, "")
	testEnv := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(testEnv)
	require.NoError(t, cfg.configureProviders(context.Background(), testStore(cfg), testEnv, resolver, providers))
	require.True(t, cfg.IsConfigured())

	large, small, err := cfg.defaultModelSelection(providers)
	require.NoError(t, err)
	require.Equal(t, primeai.Name, large.Provider)
	require.Equal(t, primeai.AutomaticModelID, large.Model)
	require.Equal(t, primeai.Name, small.Provider)
	require.Equal(t, primeai.AutomaticModelID, small.Model)
}

type mockPrimeAIAccessClient struct {
	calls    int
	provider catwalk.Provider
	access   map[string]primeai.ModelAccess
	err      error
}

func (m *mockPrimeAIAccessClient) Get(ctx context.Context) (catwalk.Provider, error) {
	p, _, err := m.GetWithAccess(ctx)
	return p, err
}

func (m *mockPrimeAIAccessClient) GetWithAccess(context.Context) (catwalk.Provider, map[string]primeai.ModelAccess, error) {
	m.calls++
	return m.provider, m.access, m.err
}

func TestPrimeAISync_RefetchIfStaleAndAccess(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	client := &mockPrimeAIAccessClient{
		provider: catwalk.Provider{
			ID:   primeai.Name,
			Name: primeai.DisplayName,
			Type: primeai.Name,
			Models: []catwalk.Model{
				{ID: primeai.AutomaticModelID, Name: primeai.AutomaticModelName},
				{ID: "m1", Name: "Model One v1"},
			},
		},
		access: map[string]primeai.ModelAccess{
			"m1": {PlanLabel: "Pro", Locked: false, AllowedTariffNames: []string{"Pro"}},
		},
	}

	syncer := &primeaiSync{now: func() time.Time { return now }}
	syncer.Init(client, filepath.Join(t.TempDir(), "primeai.json"), true)

	_, refreshed, err := syncer.RefetchIfStale(t.Context(), 20*time.Second)
	require.NoError(t, err)
	require.True(t, refreshed)
	require.Equal(t, 1, client.calls)
	acc, ok := syncer.ModelAccess("m1")
	require.True(t, ok)
	require.Equal(t, "Pro", acc.PlanLabel)
	require.False(t, acc.Locked)

	now = now.Add(10 * time.Second)
	_, refreshed, err = syncer.RefetchIfStale(t.Context(), 20*time.Second)
	require.NoError(t, err)
	require.False(t, refreshed)
	require.Equal(t, 1, client.calls)

	client.provider.Models = []catwalk.Model{
		{ID: primeai.AutomaticModelID, Name: primeai.AutomaticModelName},
		{ID: "m1", Name: "Model One v2"},
	}
	client.access = map[string]primeai.ModelAccess{
		"m1": {PlanLabel: "Ultra", Locked: true, AllowedTariffNames: []string{"Ultra"}},
	}

	now = now.Add(15 * time.Second)
	p, refreshed, err := syncer.RefetchIfStale(t.Context(), 20*time.Second)
	require.NoError(t, err)
	require.True(t, refreshed)
	require.Equal(t, 2, client.calls)
	require.Equal(t, "Model One v2", p.Models[1].Name)

	acc, ok = syncer.ModelAccess("m1")
	require.True(t, ok)
	require.Equal(t, "Ultra", acc.PlanLabel)
	require.True(t, acc.Locked)
	require.Len(t, syncer.ModelAccessMap(), 1)
}

func TestConfigStore_RefreshPrimeAIIfStaleUpdatesProvidersInPlace(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("PRIME_GLOBAL_DATA", tmpDir)
	t.Setenv("HOME", tmpDir)

	resetProviderState()
	defer resetProviderState()

	origToken := primeAccountToken
	primeAccountToken = func() string { return "acct_dyn" }
	defer func() { primeAccountToken = origToken }()

	var version int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/prime/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_dyn",
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/prime/models":
			if version == 0 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"models": []map[string]any{
						{"id": "m-pro", "name": "Old Name", "kind": "text", "planLabel": "Pro", "locked": false},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"id": "m-pro", "name": "Renamed Pro", "kind": "text", "planLabel": "Ultra", "locked": true, "allowedTariffNames": []string{"Ultra"}},
				},
			})
		}
	}))
	defer srv.Close()
	t.Setenv("PRIMEAI_URL", srv.URL)

	cfg := &Config{}
	cfg.setDefaults(tmpDir, "")
	store := testStore(cfg)

	now := time.Unix(1_700_000_000, 0)
	primeaiSyncer = &primeaiSync{now: func() time.Time { return now }}
	primeaiSyncer.Init(realPrimeAIClient{
		accountToken: primeAccountToken,
	}, filepath.Join(tmpDir, "primeai.json"), true)

	require.NoError(t, store.RefreshPrimeAIIfStale(t.Context(), 20*time.Second))

	pc, ok := store.Config().Providers.Get(primeai.Name)
	require.True(t, ok)
	require.Len(t, pc.Models, 2)
	require.Equal(t, "Old Name", pc.Models[1].Name)

	version = 1
	now = now.Add(25 * time.Second)
	require.NoError(t, store.RefreshPrimeAIIfStale(t.Context(), 20*time.Second))

	pc, ok = store.Config().Providers.Get(primeai.Name)
	require.True(t, ok)
	require.Equal(t, "Renamed Pro", pc.Models[1].Name)
	acc, ok := store.PrimeAIModelAccess("m-pro")
	require.True(t, ok)
	require.True(t, acc.Locked)
	require.Equal(t, "Ultra", acc.PlanLabel)
}
