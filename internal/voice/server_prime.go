package voice

// Helpers the idle watchdog uses before it restarts Prime: spotting a live
// Prime elsewhere on the machine, resolving the binary to re-run, and
// dropping session files that were cut mid-turn when the terminal vanished.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// processEntry is one row of a process table: a pid and its command line.
type processEntry struct {
	PID         int
	CommandLine string
}

// scanProcesses lists every process with its command line. It is a variable
// so tests can feed in a fake table.
var scanProcesses = defaultScanProcesses

// processScanTimeout bounds the OS queries; a wedged scan must not delay the
// restart decision forever.
const processScanTimeout = 8 * time.Second

// defaultScanProcesses reads the process table through Windows management
// tools: wmic first because it is quick, then PowerShell because newer
// Windows builds drop wmic. On other systems /proc carries the same facts.
func defaultScanProcesses() ([]processEntry, error) {
	if runtime.GOOS == "windows" {
		return scanProcessesWindows()
	}
	return scanProcessesProc()
}

// scanProcessesWindows tries wmic and falls back to PowerShell CIM.
func scanProcessesWindows() ([]processEntry, error) {
	wmic := exec.Command("wmic.exe", "process", "get", "ProcessId,CommandLine", "/format:csv")
	out, err := runScan(wmic)
	if err == nil {
		if entries, ok := parseCSVTable(string(out), true); ok && len(entries) > 0 {
			return entries, nil
		}
	}
	ps := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_Process | Select-Object ProcessId,CommandLine | ConvertTo-Csv")
	out, err = runScan(ps)
	if err != nil {
		return nil, err
	}
	entries, _ := parseCSVTable(string(out), false)
	return entries, nil
}

func runScan(cmd *exec.Cmd) ([]byte, error) {
	timer := time.AfterFunc(processScanTimeout, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	defer timer.Stop()
	return cmd.Output()
}

// scanProcessesProc reads /proc, which Linux and macOS-less fallbacks have;
// anything that cannot answer is reported as no data.
func scanProcessesProc() ([]processEntry, error) {
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var entries []processEntry
	for _, dir := range dirs {
		pid, err := strconv.Atoi(dir.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}
		entries = append(entries, processEntry{PID: pid, CommandLine: string(data)})
	}
	return entries, nil
}

// parseCSVTable reads a ProcessId/CommandLine table. Semicolon is the delimiter
// wmic uses for its data rows even under a comma header; comma is what
// PowerShell emits. Command lines carry the delimiter and stray quotes, so
// rows are split by hand and the command column keeps everything after it.
func parseCSVTable(text string, semicolon bool) ([]processEntry, bool) {
	delimiter := ","
	if semicolon {
		delimiter = ";"
	}
	pidColumn, cmdColumn := -1, -1
	var entries []processEntry
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, delimiter)
		if len(fields) == 1 {
			fields = strings.Split(line, otherDelimiter(delimiter))
		}
		if headerColumns(fields, &pidColumn, &cmdColumn) {
			continue
		}
		if pidColumn < 0 || cmdColumn < 0 {
			continue // no header seen yet: the columns are unknown
		}
		if pidColumn >= len(fields) || cmdColumn >= len(fields) {
			continue
		}
		pid, err := strconv.Atoi(strings.Trim(strings.TrimSpace(fields[pidColumn]), `"`))
		if err != nil {
			continue
		}
		// The command line can itself contain the delimiter, so everything
		// from its column to the end of the row belongs to it, unless a
		// later column (the pid) still needs splitting out.
		command := fields[cmdColumn]
		if cmdColumn == len(fields)-1 {
			command = strings.Join(fields[cmdColumn:], delimiter)
		}
		entries = append(entries, processEntry{
			PID:         pid,
			CommandLine: unescapeProcCmdline(cleanQuotedField(command)),
		})
	}
	if pidColumn < 0 || cmdColumn < 0 {
		return nil, false
	}
	return entries, true
}

// headerColumns learns the ProcessId and CommandLine column positions from a
// header row, reporting whether the row was one.
func headerColumns(fields []string, pidColumn, cmdColumn *int) bool {
	sawPID, sawCommand := false, false
	for i, field := range fields {
		switch strings.ToLower(strings.Trim(strings.TrimSpace(field), `"`)) {
		case "processid":
			*pidColumn, sawPID = i, true
		case "commandline":
			*cmdColumn, sawCommand = i, true
		}
	}
	return sawPID && sawCommand
}

// otherDelimiter flips the split style when wmic's comma header meets its
// semicolon rows.
func otherDelimiter(delimiter string) string {
	if delimiter == ";" {
		return ","
	}
	return ";"
}

// cleanQuotedField removes the wrapping quotes a CSV writer adds around a
// whole field, leaving interior quoting alone.
func cleanQuotedField(field string) string {
	field = strings.TrimSpace(field)
	if len(field) >= 2 && strings.HasPrefix(field, `"`) && strings.HasSuffix(field, `"`) {
		return strings.Trim(field, `"`)
	}
	return field
}

// unescapeProcCmdline turns the NUL-separated argv of /proc back into a
// command line for matching.
func unescapeProcCmdline(commandLine string) string {
	return strings.ReplaceAll(commandLine, "\x00", " ")
}

// primeRunning reports whether a live Prime other than this watchdog still
// serves the user. When the process table cannot be read the answer is
// "yes": silence is safer than launching a hidden duplicate.
func primeRunning(primeExe string, args []string) bool {
	entries, err := scanProcesses()
	if err != nil || len(entries) == 0 {
		return true
	}
	needles := []string{"prime"}
	if base := strings.ToLower(filepath.Base(primeExe)); base != "" && base != "." {
		needles = append(needles, base)
	}
	wanted := strings.ToLower(strings.Join(primeCommandArgs(args), " "))
	for _, entry := range entries {
		if entry.PID == os.Getpid() {
			continue
		}
		lower := strings.ToLower(entry.CommandLine)
		if !containsAny(lower, needles) {
			continue
		}
		if wanted != "" && !strings.Contains(lower, wanted) {
			continue
		}
		return true
	}
	return false
}

// containsAny reports whether text holds at least one of the needles.
func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

// globalFlagsWithValues lists Prime's global flags that swallow the next
// argument, so the value is dropped together with the flag.
var globalFlagsWithValues = map[string]bool{"--data-dir": true, "--cwd": true}

// primeCommandArgs strips global flags from startup arguments so a restart
// can recognise the command line it is looking for. Unknown options are
// treated as value-carrying, which keeps the result conservative.
func primeCommandArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
		case strings.HasPrefix(arg, "-"):
			name, _, hadValue := strings.Cut(arg, "=")
			if !hadValue && globalFlagsWithValues[name] {
				i++
			}
		default:
			out = append(out, arg)
		}
	}
	return out
}

// pickPrimeExe returns the first existing file among candidates.
func pickPrimeExe(candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// currentPrimeExe is this process's own binary, the fallback path for
// relaunching when the recorded command line no longer resolves.
func currentPrimeExe() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// clearInterruptedSessions deletes saved sessions left mid-turn in every
// sessions directory Prime knows about, because they cannot resume and
// relaunching would otherwise rebuild half-finished work.
func clearInterruptedSessions() {
	for _, dir := range sessionsDirs() {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			name := file.Name()
			if !strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".jsonl") {
				continue
			}
			_ = removeInterruptedSession(filepath.Join(dir, name))
		}
	}
}

// sessionsDirs lists where saved session files may live.
func sessionsDirs() []string {
	var dirs []string
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, ".prime", "sessions"))
	}
	dirs = append(dirs, filepath.Join(DefaultLayout().Root, "..", "sessions"))
	if state := os.Getenv("XDG_STATE_HOME"); state != "" {
		dirs = append(dirs, filepath.Join(state, "prime", "sessions"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "state", "prime", "sessions"))
	}
	return dirs
}

// interruptedSessionStates are saved states with no continuation: the run
// died with the terminal.
var interruptedSessionStates = map[string]bool{"running": true, "waiting_for_tools": true}

// removeInterruptedSession deletes one session file when its last state shows
// a turn that never finished. Files it cannot understand are left alone.
func removeInterruptedSession(path string) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var doc struct {
		State *string `json:"state"`
	}
	if err := json.Unmarshal(data, &doc); err == nil && doc.State != nil {
		if interruptedSessionStates[*doc.State] {
			return os.Remove(path)
		}
		return nil
	}
	// JSONL files keep one message per line; the tail carries the live state.
	if last, ok := lastJSONLine(data); ok {
		var doc struct {
			State *string `json:"state"`
		}
		if json.Unmarshal(last, &doc) == nil && doc.State != nil && interruptedSessionStates[*doc.State] {
			return os.Remove(path)
		}
	}
	return nil
}

// lastJSONLine returns the final non-empty line of a JSONL document.
func lastJSONLine(data []byte) ([]byte, bool) {
	lines := bytes.Split(bytes.TrimRight(data, " \t\r\n"), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := bytes.TrimSpace(lines[i]); len(trimmed) > 0 {
			return trimmed, true
		}
	}
	return nil, false
}
