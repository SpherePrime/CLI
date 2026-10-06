package fantasy

import "testing"

// A non-positive cap must not overwrite the settings, so a misconfigured
// value leaves the built-in default in place rather than stalling or
// crashing the fan-out.
func TestWithMaxParallelToolsIgnoresNonPositive(t *testing.T) {
	for _, n := range []int{0, -1} {
		s := &agentSettings{maxParallelTools: defaultMaxParallelTools}
		WithMaxParallelTools(n)(s)
		if s.maxParallelTools != defaultMaxParallelTools {
			t.Fatalf("WithMaxParallelTools(%d) changed the cap to %d", n, s.maxParallelTools)
		}
	}
}

// A high cap is applied verbatim so a user who wants to fan out many
// sub-agents in one message can raise it without a hard ceiling below it.
func TestWithMaxParallelToolsAppliesHighCap(t *testing.T) {
	s := &agentSettings{}
	WithMaxParallelTools(200)(s)
	if s.maxParallelTools != 200 {
		t.Fatalf("WithMaxParallelTools(200) set the cap to %d, want 200", s.maxParallelTools)
	}
}
