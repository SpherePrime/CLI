package model

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/dwertyfa288/CLI/vendordeps/bubbletea/v2"
	"github.com/dwertyfa288/CLI/internal/update"
	"github.com/dwertyfa288/CLI/internal/version"
)

type updateDoneMsg struct {
	err    error
	latest string
}

func (m *UI) performUpdate(latest string) tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			return updateDoneMsg{err: err, latest: latest}
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if _, err := update.Install(ctx, version.Version, exe, update.Default, io.Discard); err != nil {
			return updateDoneMsg{err: err, latest: latest}
		}
		return updateDoneMsg{latest: latest}
	}
}
