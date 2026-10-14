package model

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestProjectExecArgsCarriesYolo(t *testing.T) {
	t.Parallel()

	t.Run("includes yolo flag when enabled", func(t *testing.T) {
		t.Parallel()
		args := projectExecArgs("/home/user/app", true)
		require.Equal(t, []string{"--cwd", "/home/user/app", "--yolo"}, args)
	})

	t.Run("omits yolo flag when disabled", func(t *testing.T) {
		t.Parallel()
		args := projectExecArgs("/home/user/app", false)
		require.Equal(t, []string{"--cwd", "/home/user/app"}, args)
	})
}
