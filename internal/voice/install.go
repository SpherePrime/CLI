package voice

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
)

// InstallSubdir is the folder inside Prime's data directory where voice tools
// that Prime downloaded itself are kept. Keeping them out of system paths means
// no admin rights are needed and uninstalling Prime removes them with it.
const InstallSubdir = "voice"

// InstallLayout is the directory tree Prime uses for self-installed voice
// tools: binaries in Bin, Whisper models in Models.
type InstallLayout struct {
	Root string
}

// DefaultLayout returns the layout under Prime's global data directory.
func DefaultLayout() InstallLayout {
	return InstallLayout{Root: filepath.Join(config.GlobalDataDir(), InstallSubdir)}
}

// Bin returns the directory holding executables and their shared libraries.
func (l InstallLayout) Bin() string {
	return filepath.Join(l.Root, "bin")
}

// Models returns the directory holding ggml model files.
func (l InstallLayout) Models() string {
	return filepath.Join(l.Root, "models")
}

// Binary returns the path of a tool Prime installed, when it is there. This is
// how a self-installed whisper.cpp or ffmpeg is found without touching PATH.
func (l InstallLayout) Binary(name string) (string, bool) {
	if l.Root == "" {
		return "", false
	}
	candidates := []string{filepath.Join(l.Bin(), name)}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, filepath.Join(l.Bin(), name+".exe"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// ModelFile returns a downloaded model by name, for example "base" resolves to
// models/ggml-base.bin.
func (l InstallLayout) ModelFile(name string) (string, bool) {
	if l.Root == "" {
		return "", false
	}
	for _, candidate := range []string{
		filepath.Join(l.Models(), "ggml-"+name+".bin"),
		filepath.Join(l.Models(), name+".bin"),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Size() > 0 {
			return candidate, true
		}
	}
	return "", false
}

// lookPath resolves a tool from Prime's own install directory first, then from
// the system PATH. Installed tools win because setup chose them for this
// machine and they cannot be broken by a later system change.
func (l InstallLayout) lookPath(name string) (string, bool) {
	if path, ok := l.Binary(name); ok {
		return path, true
	}
	path, err := exec.LookPath(name)
	if err != nil || path == "" {
		return "", false
	}
	return path, true
}

// modelSearchDirs is where a ggml model may live: Prime's own models directory
// first, because setup chose it, then the usual whisper.cpp locations.
func (l InstallLayout) modelSearchDirs() []string {
	if l.Root == "" {
		return modelSearchDirs()
	}
	return append([]string{l.Models()}, modelSearchDirs()...)
}

// libraryPathEnv returns environment entries that let a self-installed binary
// find the shared libraries extracted next to it. Windows searches the
// executable's own directory, so only the unix loaders need help.
func (l InstallLayout) libraryPathEnv() []string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return nil
	}
	bin := l.Bin()
	variable := "LD_LIBRARY_PATH"
	current := os.Getenv(variable)
	if current != "" {
		for _, dir := range strings.Split(current, string(os.PathListSeparator)) {
			if dir == bin {
				return nil
			}
		}
		return []string{variable + "=" + bin + string(os.PathListSeparator) + current}
	}
	return []string{variable + "=" + bin}
}
