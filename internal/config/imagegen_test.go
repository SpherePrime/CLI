package config

import (
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestImageGenOptionsDefaults(t *testing.T) {
	t.Parallel()

	var options *ImageGenOptions
	require.Equal(t, DefaultImageGenModel, options.GetModel())
	require.Equal(t, DefaultImageGenBaseURL, options.GetBaseURL())
	require.Equal(t, "", options.GetOutputDir())
	require.Equal(t, "", options.GetResponseFormat())
	require.Equal(t, "", options.GetSize())
	require.Equal(t, DefaultImageGenTimeoutMs*time.Millisecond, options.GetTimeout())
	require.False(t, options.IsConfigured())
}

func TestImageGenOptionsConfigured(t *testing.T) {
	t.Parallel()

	options := &ImageGenOptions{
		APIKey:         "$MY_API_KEY",
		Model:          " dall-e-3 ",
		BaseURL:        "https://example.com/v1/",
		OutputDir:      "./pictures",
		ResponseFormat: "url",
		Size:           "1024x1024",
		TimeoutMs:      60_000,
	}
	require.Equal(t, "dall-e-3", options.GetModel())
	require.Equal(t, "https://example.com/v1", options.GetBaseURL())
	require.Equal(t, "./pictures", options.GetOutputDir())
	require.Equal(t, "url", options.GetResponseFormat())
	require.Equal(t, "1024x1024", options.GetSize())
	require.Equal(t, 60_000*time.Millisecond, options.GetTimeout())
	require.True(t, options.IsConfigured())
}

func TestImageGenOptionsInvalidResponseFormatIsDropped(t *testing.T) {
	t.Parallel()

	options := &ImageGenOptions{ResponseFormat: "png"}
	require.Equal(t, "", options.GetResponseFormat())
}
