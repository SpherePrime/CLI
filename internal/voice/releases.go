package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// whisperCPPAPIURL lists the prebuilt Whisper binaries. Releases are queried
// rather than hardcoded because whisper.cpp publishes its binaries in a rolling
// nightly release, and a pinned version rots.
const whisperCPPAPIURL = "https://api.github.com/repos/ggml-org/whisper.cpp/releases?per_page=15"

// modelsURL is where whisper.cpp keeps its published ggml models.
const modelsURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

// SetupModels lists the Whisper model names that can be downloaded, with the
// approximate size so the plan can say what it costs. The default comes first.
var SetupModels = []struct {
	Name string
	MB   int
	Note string
}{
	{Name: "large-v3-turbo-q5_0", MB: 574, Note: "the default: accurate multilingual model that still runs on a CPU"},
	{Name: "tiny", MB: 75, Note: "fastest, weakest accuracy"},
	{Name: "base", MB: 142, Note: "good balance, multilingual"},
	{Name: "small", MB: 465, Note: "smaller than the default, faster on old hardware"},
	{Name: "medium", MB: 1500, Note: "needs a strong GPU"},
}

// DefaultSetupModel is the model Prime downloads when none is chosen: Whisper
// Large V3 Turbo quantized to q5_0, which is the best accuracy that still
// transcribes dictation in real time on a laptop CPU. It is multilingual,
// which matters for dictation in the languages Prime's UI speaks.
const DefaultSetupModel = "large-v3-turbo-q5_0"

// releaseAsset is one downloadable file attached to a GitHub release.
type releaseAsset struct {
	Name                 string `json:"name"`
	URL                  string `json:"browser_download_url"`
	Size                 int64  `json:"size"`
	Digest               string `json:"digest"`
	ContentLengthUnknown bool   `json:"-"`
}

// release is a GitHub release with the assets Prime cares about.
type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// assetNameForPlatform returns the whisper.cpp release asset that carries a
// command line binary for a platform. macOS is absent on purpose: upstream
// ships only a framework there, so `brew install whisper-cpp` remains the way.
func assetNameForPlatform(goos, goarch string) (string, bool) {
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "whisper-bin-x64.zip", true
	case "windows/386":
		return "whisper-bin-Win32.zip", true
	case "windows/arm64":
		return "whisper-bin-win-cpu-arm64.zip", true
	case "linux/amd64":
		return "whisper-bin-ubuntu-x64.tar.gz", true
	case "linux/arm64":
		return "whisper-bin-ubuntu-arm64.tar.gz", true
	}
	return "", false
}

// platformAssetName is assetNameForPlatform for the machine Prime runs on.
func platformAssetName() (string, bool) {
	return assetNameForPlatform(runtime.GOOS, runtime.GOARCH)
}

// releasesClient fetches the release list. apiURL is a parameter so tests can
// serve a canned list instead of depending on GitHub.
func releasesClient(ctx context.Context, client *http.Client, apiURL string) ([]release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "prime-voice-setup")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the whisper.cpp release list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper.cpp release list returned %s", resp.Status)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("unexpected release list: %w", err)
	}
	return releases, nil
}

// pickAsset finds the newest release carrying the asset for this platform.
// GitHub returns releases newest first, so the first hit is the freshest build.
func pickAsset(releases []release, assetName string) (releaseAsset, string, bool) {
	for _, entry := range releases {
		for _, asset := range entry.Assets {
			if asset.Name == assetName {
				return asset, entry.TagName, true
			}
		}
	}
	return releaseAsset{}, "", false
}

// assetSHA256 extracts the hex digest from an asset's "sha256:..." field.
func assetSHA256(asset releaseAsset) string {
	_, hex, found := strings.Cut(asset.Digest, ":")
	if !found {
		return ""
	}
	return hex
}

// modelFileName is the ggml file for a model name.
func modelFileName(name string) string {
	return "ggml-" + name + ".bin"
}

// modelURLFor returns the download URL of a model name.
func modelURLFor(name string) string {
	return modelsURL + "/" + modelFileName(name)
}

// setupModelKnown reports whether a model name can be downloaded.
func setupModelKnown(name string) bool {
	for _, model := range SetupModels {
		if model.Name == name {
			return true
		}
	}
	return false
}

// modelSizeMB returns the advertised size of a model name.
func modelSizeMB(name string) int {
	for _, model := range SetupModels {
		if model.Name == name {
			return model.MB
		}
	}
	return 0
}
