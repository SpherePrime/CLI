package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestTUIOptionsShimmerDefaultsOn(t *testing.T) {
	t.Parallel()

	require.True(t, (*TUIOptions)(nil).IsShimmer(), "no options at all must keep the shimmer")
	require.True(t, (&TUIOptions{}).IsShimmer(), "an unset option must keep the shimmer")
	require.False(t, (&TUIOptions{Shimmer: ptr(false)}).IsShimmer())
	require.True(t, (&TUIOptions{Shimmer: ptr(true)}).IsShimmer())
}
