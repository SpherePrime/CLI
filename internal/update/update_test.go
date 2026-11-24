package update

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCheckForUpdate_Old(t *testing.T) {
	info, err := Check(t.Context(), "v0.10.0", testClient{"v0.11.0"})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Available())
}

func TestCheckForUpdate_Beta(t *testing.T) {
	t.Run("current is stable", func(t *testing.T) {
		info, err := Check(t.Context(), "v0.10.0", testClient{"v0.11.0-beta.1"})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.False(t, info.Available())
	})

	t.Run("current is also beta", func(t *testing.T) {
		info, err := Check(t.Context(), "v0.11.0-beta.1", testClient{"v0.11.0-beta.2"})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.True(t, info.Available())
	})

	t.Run("current is beta, latest isn't", func(t *testing.T) {
		info, err := Check(t.Context(), "v0.11.0-beta.1", testClient{"v0.11.0"})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.True(t, info.Available())
	})
}

func TestCheckForUpdate_RejectsDowngrade(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
	}{
		{name: "newer stable than latest", current: "v0.12.0", latest: "v0.11.0"},
		{name: "equal version", current: "v0.11.0", latest: "v0.11.0"},
		{name: "newer prerelease than stable", current: "v0.12.0-beta.1", latest: "v0.11.0"},
		{name: "newer beta than older beta", current: "v0.11.0-beta.10", latest: "v0.11.0-beta.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := Check(t.Context(), tc.current, testClient{tc.latest})
			require.NoError(t, err)
			require.False(t, info.Available())
		})
	}
}

func TestCheckForUpdate_DevBuilds(t *testing.T) {
	cases := []string{
		"devel",
		"unknown",
		"v0.10.0-dirty",
		"v0.0.0-0.20251231235959-06c807842604",
	}
	for _, current := range cases {
		t.Run(current, func(t *testing.T) {
			info, err := Check(t.Context(), current, testClient{"v0.11.0"})
			require.NoError(t, err)
			require.True(t, info.IsDevelopment())
			require.False(t, info.Available())
		})
	}
}

type countingClient struct {
	tag   string
	calls int
}

func (c *countingClient) Latest(ctx context.Context) (*Release, error) {
	c.calls++
	return &Release{
		TagName: c.tag,
		HTMLURL: "https://example.org",
	}, nil
}

func TestInstallChecked_ReusesReleaseMetadata(t *testing.T) {
	client := &countingClient{tag: "v0.11.0"}
	info, err := Check(t.Context(), "v0.10.0", client)
	require.NoError(t, err)
	require.Equal(t, 1, client.calls)
	_, err = InstallChecked(t.Context(), info, "prime", client, nil)
	require.Error(t, err)
	require.Equal(t, 1, client.calls)
}

type testClient struct{ tag string }

// Latest implements Client.
func (t testClient) Latest(ctx context.Context) (*Release, error) {
	return &Release{
		TagName: t.tag,
		HTMLURL: "https://example.org",
	}, nil
}
