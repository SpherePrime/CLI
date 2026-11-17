package tools

import (
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestCreateFileTree(t *testing.T) {
	t.Parallel()

	root := filepath.FromSlash("/repo")
	paths := []string{
		filepath.FromSlash("/repo/cmd/"),
		filepath.FromSlash("/repo/cmd/main.go"),
		filepath.FromSlash("/repo/internal/"),
		filepath.FromSlash("/repo/internal/pkg/"),
		filepath.FromSlash("/repo/internal/pkg/helper.go"),
		filepath.FromSlash("/repo/README.md"),
	}

	tree := createFileTree(paths, root)
	require.Len(t, tree, 3)

	require.Equal(t, "cmd", tree[0].Name)
	require.Equal(t, NodeTypeDirectory, tree[0].Type)
	require.Len(t, tree[0].Children, 1)
	require.Equal(t, "main.go", tree[0].Children[0].Name)
	require.Equal(t, NodeTypeFile, tree[0].Children[0].Type)
	require.Equal(t, filepath.FromSlash("cmd/main.go"), tree[0].Children[0].Path)

	require.Equal(t, "internal", tree[1].Name)
	require.Len(t, tree[1].Children, 1)
	require.Equal(t, "pkg", tree[1].Children[0].Name)
	require.Equal(t, filepath.FromSlash("internal/pkg"), tree[1].Children[0].Path)
	require.Len(t, tree[1].Children[0].Children, 1)
	require.Equal(t, "helper.go", tree[1].Children[0].Children[0].Name)
	require.Equal(t, filepath.FromSlash("internal/pkg/helper.go"), tree[1].Children[0].Children[0].Path)

	require.Equal(t, "README.md", tree[2].Name)
	require.Equal(t, NodeTypeFile, tree[2].Type)

	rendered := printTree(tree, root)
	require.Contains(t, rendered, "- /repo/\n")
	require.Contains(t, rendered, "  - cmd/\n")
	require.Contains(t, rendered, "    - main.go\n")
	require.Contains(t, rendered, "  - README.md\n")
}
