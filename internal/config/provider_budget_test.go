package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestAwaitCatalogReturnsFetchedResult(t *testing.T) {
	t.Parallel()

	done := make(chan catalogResult[string], 1)
	done <- catalogResult[string]{value: "fresh", err: errors.New("cache write")}
	result := awaitCatalog(done, make(chan struct{}), func() string { return "cached" })
	require.Equal(t, "fresh", result.value)
	require.EqualError(t, result.err, "cache write")
}

func TestAwaitCatalogFallsBackAfterBudget(t *testing.T) {
	t.Parallel()

	budget, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	done := make(chan catalogResult[string], 1)
	started := time.Now()
	first := awaitCatalog(done, budget.Done(), func() string { return "cached" })
	second := awaitCatalog(done, budget.Done(), func() string { return "embedded" })
	require.Equal(t, "cached", first.value)
	require.Equal(t, "embedded", second.value)
	require.NoError(t, first.err)
	require.Less(t, time.Since(started), time.Second)
}
