package projects

import (
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// The system clock is coarse on Windows and time.Now().UTC() drops the
// monotonic reading, so two registrations can land on the same timestamp.
// slices.SortFunc is not stable, so without an explicit rule the project the
// user just opened could sort behind an older one.
func TestSortMostRecentFirstBreaksTimestampTies(t *testing.T) {
	t.Parallel()

	tie := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)

	t.Run("just registered project wins the tie", func(t *testing.T) {
		t.Parallel()

		projects := []Project{
			{Path: "/home/user/project1", LastAccessed: tie},
			{Path: "/home/user/project2", LastAccessed: tie},
		}

		sortMostRecentFirst(projects, "/home/user/project2")
		require.Equal(t, "/home/user/project2", projects[0].Path)

		sortMostRecentFirst(projects, "/home/user/project1")
		require.Equal(t, "/home/user/project1", projects[0].Path)
	})

	t.Run("equal timestamps sort reproducibly", func(t *testing.T) {
		t.Parallel()

		// Nothing is "most recent", so the order must come from the paths
		// alone and match on every run.
		want := []string{"/a", "/b", "/c"}
		for range 20 {
			projects := []Project{
				{Path: "/c", LastAccessed: tie},
				{Path: "/a", LastAccessed: tie},
				{Path: "/b", LastAccessed: tie},
			}

			sortMostRecentFirst(projects, "")

			got := make([]string, len(projects))
			for i, p := range projects {
				got[i] = p.Path
			}
			require.Equal(t, want, got)
		}
	})

	t.Run("timestamp ordering is preserved", func(t *testing.T) {
		t.Parallel()

		projects := []Project{
			{Path: "/home/user/project2", LastAccessed: tie},
			{Path: "/home/user/project1", LastAccessed: tie.Add(-time.Hour)},
		}

		// Paths sort project1 before project2, so this only holds if the
		// timestamps are actually compared.
		sortMostRecentFirst(projects, "")
		require.Equal(t, "/home/user/project2", projects[0].Path)

		// And the order reverses once the timestamps swap, whatever the paths.
		projects[0].LastAccessed = tie.Add(-time.Hour)
		projects[1].LastAccessed = tie
		sortMostRecentFirst(projects, "")
		require.Equal(t, "/home/user/project1", projects[0].Path)
	})

	t.Run("unknown most recent path leaves the order alone", func(t *testing.T) {
		t.Parallel()

		projects := []Project{
			{Path: "/home/user/old", LastAccessed: tie.Add(-time.Hour)},
			{Path: "/home/user/new", LastAccessed: tie},
		}

		sortMostRecentFirst(projects, "/home/user/missing")
		require.Equal(t, "/home/user/new", projects[0].Path)
	})
}

// Register has to keep that promise through a real save and load, including
// the JSON round trip that strips any monotonic clock reading.
func TestRegisterKeepsJustOpenedProjectFirst(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("PRIME_GLOBAL_DATA", tmpDir+"/prime")

	for _, path := range []string{"/home/user/a", "/home/user/b", "/home/user/c"} {
		require.NoError(t, Register(path, path+"/.prime"))
	}

	// Re-open the oldest one. Even if every timestamp ties, it must lead.
	require.NoError(t, Register("/home/user/a", "/home/user/a/.prime"))

	projects, err := List()
	require.NoError(t, err)
	require.Len(t, projects, 3)
	require.Equal(t, "/home/user/a", projects[0].Path)
}
