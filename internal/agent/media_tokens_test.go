package agent

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))))
	return buffer.Bytes()
}

func TestImageTokensUseDimensions(t *testing.T) {
	require.Equal(t, int64(1334), estimateMediaTokens("image/png", "", pngBytes(t, 1000, 1000)))
	require.Equal(t, int64(85), estimateMediaTokens("image/png", "", pngBytes(t, 50, 50)))
	require.Equal(t, int64(1534), estimateMediaTokens("image/png", "", pngBytes(t, 3000, 2000)))
}

func TestBase64ImageTokensUseDimensions(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(pngBytes(t, 1000, 1000))
	require.Equal(t, int64(1334), estimateBase64MediaTokens("image/png", "", encoded))
}

func TestUnknownImageUsesConservativeEstimate(t *testing.T) {
	require.Equal(t, int64(imageUnknownSizeTokens), estimateMediaTokens("image/webp", "", []byte("not an image")))
}

func TestPDFTokensCountPages(t *testing.T) {
	pdf := []byte("/Type /Pages /Type /Page x /Type /Page y /Type /Page")
	require.Equal(t, int64(3*documentTokensPerPage), estimateMediaTokens("application/pdf", "", pdf))
}

func TestContextCalibrationRejectsOutliers(t *testing.T) {
	ratio, ok := contextCalibrationRatio(1200, 1000)
	require.True(t, ok)
	require.InDelta(t, 1.2, ratio, 0.0001)
	_, ok = contextCalibrationRatio(100000, 1000)
	require.False(t, ok)
	categories, total := calibrateContextCategories([]ContextCategory{{Key: "tools", Tokens: 100}, {Key: "tool_results", Tokens: 300}}, 1.5)
	require.Equal(t, int64(600), total)
	require.Equal(t, int64(150), categories[0].Tokens)
}
