package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// pointer to an int literal.
func mpi(v int) *int { return &v }

// GetMaxParallelTools is the upper bound on how many parallel tool calls -
// and with it sub-agents launched in one message - may run at once. An unset
// value resolves to the default, and there is no upper bound: a positive
// value is honored verbatim so the fan-out can grow as large as the user
// wants.
func TestGetMaxParallelTools(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   *Options
		want int
	}{
		{"nil options", nil, DefaultMaxParallelTools},
		{"unset", &Options{}, DefaultMaxParallelTools},
		{"zero is the default", &Options{MaxParallelTools: mpi(0)}, DefaultMaxParallelTools},
		{"one is the floor", &Options{MaxParallelTools: mpi(1)}, 1},
		{"in range", &Options{MaxParallelTools: mpi(50)}, 50},
		{"a high value is honored", &Options{MaxParallelTools: mpi(200)}, 200},
		{"there is no ceiling", &Options{MaxParallelTools: mpi(1000)}, 1000},
		{"negative resolves to the default", &Options{MaxParallelTools: mpi(-5)}, DefaultMaxParallelTools},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.in.GetMaxParallelTools())
		})
	}
}

// A value exactly at the default must round-trip without being clamped.
func TestGetMaxParallelToolsDefaultPassthrough(t *testing.T) {
	t.Parallel()
	require.Equal(t, DefaultMaxParallelTools,
		(&Options{MaxParallelTools: mpi(DefaultMaxParallelTools)}).GetMaxParallelTools())
}
