package tools

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/SpherePrime/CLI/internal/log"
)

var getRg = sync.OnceValue(func() string {
	if testing.Testing() {
		return ""
	}
	binName := "rg"
	if runtime.GOOS == "windows" {
		binName = "rg.exe"
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		for _, candidate := range []string{
			filepath.Join(exeDir, binName),
			filepath.Join(exeDir, "bin", binName),
			filepath.Join(exeDir, "engine", binName),
		} {
			if isExecutableFile(candidate) {
				return candidate
			}
		}
	}
	path, err := exec.LookPath("rg")
	if err != nil {
		if log.Initialized() {
			slog.Warn("Ripgrep (rg) not found in $PATH. Some grep features might be limited or slower.")
		}
		return ""
	}
	return path
})

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func getRgCmd(ctx context.Context, globPattern string) *exec.Cmd {
	name := getRg()
	if name == "" {
		return nil
	}
	// Do not pass -L so glob stays scoped to the search root and avoids symlink cycles.
	args := []string{"--files", "--null"}
	if globPattern != "" {
		globPattern = filepath.ToSlash(globPattern)
		if !filepath.IsAbs(globPattern) && !strings.HasPrefix(globPattern, "/") {
			globPattern = "/" + globPattern
		}
		args = append(args, "--glob", globPattern)
	}
	return exec.CommandContext(ctx, name, args...)
}

func getRgSearchCmd(ctx context.Context, pattern, path, include string) *exec.Cmd {
	name := getRg()
	if name == "" {
		return nil
	}
	args := []string{"--json", "-H", "-n", "-0", "--max-filesize", "2M"}
	if include != "" {
		args = append(args, "--glob", strings.TrimPrefix(filepath.ToSlash(include), "./"))
	}
	for _, ignoreFile := range []string{".gitignore", ".primeignore"} {
		ignorePath := filepath.Join(path, ignoreFile)
		if _, err := os.Stat(ignorePath); err == nil {
			args = append(args, "--ignore-file", ignorePath)
		}
	}
	args = append(args, "--", pattern, path)

	return exec.CommandContext(ctx, name, args...)
}
