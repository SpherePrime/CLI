package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	BlenderMCPName             = "blender"
	blenderEnvironmentCacheTTL = 60 * time.Second
)

type BlenderEnvironment struct {
	Installed      bool
	ExecutablePath string
	MCPCommand     string
	MCPArgs        []string
	MCPAvailable   bool
}

var (
	blenderEnvDetector = detectBlenderEnvironmentLive
	blenderEnvCache    struct {
		mu        sync.Mutex
		env       BlenderEnvironment
		checkedAt time.Time
		known     bool
	}
)

func DetectBlenderEnvironment() BlenderEnvironment {
	blenderEnvCache.mu.Lock()
	if blenderEnvCache.known && time.Since(blenderEnvCache.checkedAt) <= blenderEnvironmentCacheTTL {
		env := cloneBlenderEnv(blenderEnvCache.env)
		blenderEnvCache.mu.Unlock()
		return env
	}
	blenderEnvCache.mu.Unlock()

	return RefreshBlenderEnvironment()
}

func RefreshBlenderEnvironment() BlenderEnvironment {
	env := blenderEnvDetector()
	blenderEnvCache.mu.Lock()
	blenderEnvCache.env = cloneBlenderEnv(env)
	blenderEnvCache.checkedAt = time.Now()
	blenderEnvCache.known = true
	blenderEnvCache.mu.Unlock()
	return env
}

func cloneBlenderEnv(env BlenderEnvironment) BlenderEnvironment {
	if len(env.MCPArgs) > 0 {
		copied := make([]string, len(env.MCPArgs))
		copy(copied, env.MCPArgs)
		env.MCPArgs = copied
	}
	return env
}

func detectBlenderEnvironmentLive() BlenderEnvironment {
	exe := detectBlenderExecutable()
	cmd, args := resolveBlenderMCPCommand()
	return BlenderEnvironment{
		Installed:      exe != "",
		ExecutablePath: exe,
		MCPCommand:     cmd,
		MCPArgs:        args,
		MCPAvailable:   cmd != "",
	}
}

func detectBlenderExecutable() string {
	if path, err := exec.LookPath("blender"); err == nil && path != "" {
		return path
	}
	switch runtime.GOOS {
	case "windows":
		var candidates []string
		for _, root := range []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs"),
		} {
			if strings.TrimSpace(root) == "" {
				continue
			}
			matches, _ := filepath.Glob(filepath.Join(root, "Blender Foundation", "Blender*", "blender.exe"))
			candidates = append(candidates, matches...)
		}
		if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
			steamPath := filepath.Join(pf86, "Steam", "steamapps", "common", "Blender", "blender.exe")
			if fileExists(steamPath) {
				candidates = append(candidates, steamPath)
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
		for _, candidate := range candidates {
			if fileExists(candidate) {
				return candidate
			}
		}
	case "darwin":
		for _, candidate := range []string{
			"/Applications/Blender.app/Contents/MacOS/Blender",
			filepath.Join(userHomeDir(), "Applications", "Blender.app", "Contents", "MacOS", "Blender"),
		} {
			if fileExists(candidate) {
				return candidate
			}
		}
	default:
		for _, candidate := range []string{
			"/usr/bin/blender",
			"/usr/local/bin/blender",
			"/snap/bin/blender",
			"/var/lib/flatpak/exports/bin/org.blender.Blender",
		} {
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func resolveBlenderMCPCommand() (string, []string) {
	home := userHomeDir()
	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}

	if path, err := exec.LookPath("mcp-for-blender"); err == nil && path != "" {
		return path, nil
	}
	if home != "" {
		localBin := filepath.Join(home, ".local", "bin", "mcp-for-blender"+exeSuffix)
		if fileExists(localBin) {
			return localBin, nil
		}
	}

	if path, err := exec.LookPath("uvx"); err == nil && path != "" {
		return path, []string{"mcp-for-blender"}
	}
	if home != "" {
		for _, candidate := range []string{
			filepath.Join(home, ".local", "bin", "uvx"+exeSuffix),
			filepath.Join(home, ".cargo", "bin", "uvx"+exeSuffix),
		} {
			if fileExists(candidate) {
				return candidate, []string{"mcp-for-blender"}
			}
		}
	}

	if path, err := exec.LookPath("pipx"); err == nil && path != "" {
		return path, []string{"run", "mcp-for-blender"}
	}

	return "", nil
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func BlenderMCPConfigFromEnv(env BlenderEnvironment) MCPConfig {
	cmd := env.MCPCommand
	args := env.MCPArgs
	if cmd == "" {
		cmd = "uvx"
		args = []string{"mcp-for-blender"}
	}
	copiedArgs := make([]string, len(args))
	copy(copiedArgs, args)
	return MCPConfig{
		Type:    MCPStdio,
		Command: cmd,
		Args:    copiedArgs,
		Env: map[string]string{
			"PYTHONUTF8":       "1",
			"PYTHONIOENCODING": "utf-8",
		},
		Timeout:  30,
		Disabled: false,
	}
}

func (c *Config) addDesktopBlenderMCP() {
	url, token := desktopBrowserCredentials()
	if url == "" || token == "" {
		return
	}
	if c.MCP == nil {
		c.MCP = make(map[string]MCPConfig)
	}
	if _, exists := c.MCP[BlenderMCPName]; exists {
		return
	}
	env := DetectBlenderEnvironment()
	if !env.MCPAvailable {
		return
	}
	c.MCP[BlenderMCPName] = BlenderMCPConfigFromEnv(env)
}

func (c *Config) IsBlenderMCPEnabled() bool {
	if c == nil || c.MCP == nil {
		return false
	}
	cfg, exists := c.MCP[BlenderMCPName]
	return exists && !cfg.Disabled
}
