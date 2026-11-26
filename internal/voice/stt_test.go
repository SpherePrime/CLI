package voice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStatusReportsInstalledWhenAssetsPresent(t *testing.T) {
	dataDir := t.TempDir()
	asrDir := asrDataDir(dataDir)
	require.NoError(t, os.MkdirAll(asrDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(asrDir, asrBinaryName()), []byte("bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(asrDir, asrModelFile), []byte("asr"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(asrDir, vadModelFile), []byte("vad"), 0o644))

	t.Setenv("PRIME_ASR_DIR", asrDir)
	status := Status(dataDir, "ru-RU")
	require.Equal(t, EngineTinyASR, status.Engine)
	require.Equal(t, "en-ru-uk", status.Model)
	require.Equal(t, "ru", status.Language)
	require.Equal(t, AllowedLanguages, status.Languages)
	require.True(t, status.Installed)
}

func TestExtractPcmFromWav(t *testing.T) {
	pcm := make([]byte, 3200)
	pcm[0] = 0x12
	pcm[1] = 0x34
	wav := encodeWAV(pcm)
	extracted, err := extractPcmS16LE(wav)
	require.NoError(t, err)
	require.Equal(t, pcm, extracted)
}

func TestTinyAsrGrpcRoundTripWhenBundled(t *testing.T) {
	localAsrDir := filepath.Join("..", "..", "desktop", "src-tauri", "asr")
	if !nonEmptyFile(filepath.Join(localAsrDir, asrBinaryName())) ||
		!nonEmptyFile(filepath.Join(localAsrDir, asrModelFile)) ||
		!nonEmptyFile(filepath.Join(localAsrDir, vadModelFile)) {
		t.Skip("local prime-asr bundle not prepared")
	}

	t.Setenv("PRIME_ASR_DIR", localAsrDir)
	defer CloseAsr()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	transcriber := newTinyAsrTranscriber(Settings{DataDir: t.TempDir(), Language: "auto"})
	silence := make([]byte, SampleRate*2)
	text, err := transcriber.Transcribe(ctx, &Audio{
		WAV:      encodeWAV(silence),
		Duration: time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, "", text)

	live, err := StartLiveStream(ctx, t.TempDir(), "auto", "ru", nil)
	require.NoError(t, err)
	require.NoError(t, live.PushPCM(silence[:3200]))
	finalText, err := live.Finish()
	require.NoError(t, err)
	require.Equal(t, "", finalText)
}
