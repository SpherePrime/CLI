package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// releaseListServer serves a canned whisper.cpp release list.
func releaseListServer(t *testing.T, tag, assetName, digest string, size int64) *httptest.Server {
	t.Helper()

	body, err := json.Marshal([]release{{
		TagName: tag,
		Assets: []releaseAsset{{
			Name:   assetName,
			URL:    "http://ignored/" + assetName,
			Size:   size,
			Digest: digest,
		}},
	}})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestPickAssetFindsMatchingRelease(t *testing.T) {
	t.Parallel()

	server := releaseListServer(t, "b5130", "whisper-bin-x64.zip", "sha256:abc", 8<<20)
	defer server.Close()

	releases, err := releasesClient(context.Background(), server.Client(), server.URL)
	require.NoError(t, err)

	asset, tag, ok := pickAsset(releases, "whisper-bin-x64.zip")
	require.True(t, ok)
	require.Equal(t, "b5130", tag)
	require.Equal(t, "abc", assetSHA256(asset))
}

func TestAssetNameForPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos   string
		goarch string
		want   string
		found  bool
	}{
		{"windows", "amd64", "whisper-bin-x64.zip", true},
		{"windows", "386", "whisper-bin-Win32.zip", true},
		{"windows", "arm64", "whisper-bin-win-cpu-arm64.zip", true},
		{"linux", "amd64", "whisper-bin-ubuntu-x64.tar.gz", true},
		{"linux", "arm64", "whisper-bin-ubuntu-arm64.tar.gz", true},
		{"darwin", "arm64", "", false},
	}

	for _, tc := range tests {
		name, found := assetNameForPlatform(tc.goos, tc.goarch)
		require.Equal(t, tc.found, found, tc.goos+"/"+tc.goarch)
		require.Equal(t, tc.want, name, tc.goos+"/"+tc.goarch)
	}
}

// emptyProberWithLayout is emptyProber with a real layout, so plans get a
// place to look.
func emptyProberWithLayout(t *testing.T) prober {
	p := emptyProber()
	p.layout = InstallLayout{Root: t.TempDir()}
	return p
}

// forceEngineMissing makes the prober report a machine with no usable engine,
// so a plan for the whisper.cpp engine is produced.
func forceEngineMissing(t *testing.T) prober {
	p := emptyProberWithLayout(t)
	return p
}

func TestSetupPlanOnPlatformWithoutAsset(t *testing.T) {
	t.Parallel()

	if _, ok := assetNameForPlatform(runtime.GOOS, runtime.GOARCH); ok {
		t.Skip("this platform has a prebuilt asset, so the plan cannot say it does not")
	}

	p := forceEngineMissing(t)
	plan, err := planSetup(context.Background(), Settings{}, p, SetupOptions{})
	require.NoError(t, err)
	require.True(t, plan.Empty())
	require.Contains(t, plan.Steps(), "no prebuilt Whisper binaries")
}

func TestSetupPlanInstallsEngineAndModelWhenMissing(t *testing.T) {
	t.Parallel()

	if _, ok := assetNameForPlatform(runtime.GOOS, runtime.GOARCH); !ok {
		t.Skip("no prebuilt asset for this platform")
	}
	assetName, _ := assetNameForPlatform(runtime.GOOS, runtime.GOARCH)
	server := releaseListServer(t, "b5130", assetName, "sha256:abc", 8<<20)
	defer server.Close()

	p := emptyProberWithLayout(t)
	plan, err := planSetup(context.Background(), Settings{}, p, SetupOptions{ReleasesURL: server.URL})
	require.NoError(t, err)
	require.NotNil(t, plan.Engine)
	require.Equal(t, assetName, plan.Engine.Name)
	require.Equal(t, DefaultSetupModel, plan.Model)
	require.Equal(t, modelURLFor(DefaultSetupModel), plan.ModelURL)
	require.True(t, plan.NeedsApproval())
}

func TestSetupPlanHonorsCustomModel(t *testing.T) {
	t.Parallel()

	if _, ok := assetNameForPlatform(runtime.GOOS, runtime.GOARCH); !ok {
		t.Skip("no prebuilt asset for this platform")
	}
	assetName, _ := assetNameForPlatform(runtime.GOOS, runtime.GOARCH)
	server := releaseListServer(t, "b5130", assetName, "sha256:abc", 8<<20)
	defer server.Close()

	p := emptyProberWithLayout(t)
	plan, err := planSetup(context.Background(), Settings{}, p, SetupOptions{
		ReleasesURL: server.URL,
		Model:       "small",
	})
	require.NoError(t, err)
	require.Equal(t, "small", plan.Model)
	require.Equal(t, modelURLFor("small"), plan.ModelURL)
}

func TestSetupPlanRejectsUnknownModel(t *testing.T) {
	t.Parallel()

	if _, ok := assetNameForPlatform(runtime.GOOS, runtime.GOARCH); !ok {
		t.Skip("no prebuilt asset for this platform")
	}
	assetName, _ := assetNameForPlatform(runtime.GOOS, runtime.GOARCH)
	server := releaseListServer(t, "b5130", assetName, "sha256:abc", 8<<20)
	defer server.Close()

	p := emptyProberWithLayout(t)
	_, err := planSetup(context.Background(), Settings{}, p, SetupOptions{
		ReleasesURL: server.URL,
		Model:       "enormous",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown model")
}

func TestSetupPlanSkipsEngineWhenAlreadyAvailable(t *testing.T) {
	t.Parallel()

	// A working whisper.cpp with a model in the layout means no engine or
	// model download. The recorder is a separate concern, so its absence
	// still shows in the plan.
	bin := filepath.Join(t.TempDir(), whisperCPPBinary)
	_ = os.WriteFile(bin, []byte("fake"), 0o755)

	p := emptyProberWithLayout(t)
	p.lookPath = func(name string) (string, bool) {
		if name == whisperCPPBinary {
			return bin, true
		}
		return "", false
	}
	modelDir := filepath.Join(p.layout.Root, "models")
	_ = os.MkdirAll(modelDir, 0o755)
	_ = os.WriteFile(filepath.Join(modelDir, "ggml-base.bin"), []byte("weights"), 0o600)

	plan, err := planSetup(context.Background(), Settings{}, p, SetupOptions{})
	require.NoError(t, err)
	require.Nil(t, plan.Engine, "engine must not be re-downloaded when one is present")
	require.Empty(t, plan.Model, "model must not be re-downloaded when one is present")
}

func TestSetupPlanIsIdempotent(t *testing.T) {
	t.Parallel()

	if _, ok := assetNameForPlatform(runtime.GOOS, runtime.GOARCH); !ok {
		t.Skip("no prebuilt asset for this platform")
	}
	assetName, _ := assetNameForPlatform(runtime.GOOS, runtime.GOARCH)
	server := releaseListServer(t, "b5130", assetName, "sha256:abc", 8<<20)
	defer server.Close()

	p := emptyProberWithLayout(t)
	binDir := p.layout.Bin()
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.WriteFile(filepath.Join(binDir, whisperCPPBinary), []byte("fake"), 0o755)
	modelDir := filepath.Join(p.layout.Root, "models")
	_ = os.MkdirAll(modelDir, 0o755)
	_ = os.WriteFile(filepath.Join(modelDir, "ggml-base.bin"), []byte("weights"), 0o600)
	p.lookPath = func(name string) (string, bool) {
		if name == whisperCPPBinary {
			return filepath.Join(binDir, whisperCPPBinary), true
		}
		return "", false
	}

	plan, err := planSetup(context.Background(), Settings{}, p, SetupOptions{ReleasesURL: server.URL})
	require.NoError(t, err)
	require.Nil(t, plan.Engine)
	require.Empty(t, plan.Model)
}
