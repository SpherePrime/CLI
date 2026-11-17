package customagents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/internal/config"
)

const (
	DefaultMemoryCharLimit = 2200
	DefaultUserCharLimit   = 1375
	EntryDelimiter         = "\n§\n"

	memoryFileName = "MEMORY.md"
	userFileName   = "USER.md"
	soulFileName   = "SOUL.md"
)

var (
	injectionPhrases = []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"ignore prior instructions",
		"disregard rules",
		"disregard all rules",
		"disregard previous",
		"system prompt",
		"override instructions",
		"forget your instructions",
		"reveal your instructions",
		"bypass safety",
		"<system>",
		"</system>",
		"<|system|>",
		"<|im_start|>",
	}

	secretPatterns = []struct {
		label string
		re    *regexp.Regexp
	}{
		{label: "sk- token", re: regexp.MustCompile(`(?:^|[^A-Za-z0-9])sk-[A-Za-z0-9_-]{4,}`)},
		{label: "git token", re: regexp.MustCompile(`(?:^|[^A-Za-z0-9])(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{4,}`)},
		{label: "git pat", re: regexp.MustCompile(`(?:^|[^A-Za-z0-9])github_pat_[A-Za-z0-9_]{8,}`)},
		{label: "slack token", re: regexp.MustCompile(`(?:^|[^A-Za-z0-9])xox[bpa]-[A-Za-z0-9-]{4,}`)},
		{label: "api key", re: regexp.MustCompile(`(?:^|[^A-Za-z0-9])AIza[A-Za-z0-9_-]{4,}`)},
		{label: "access key", re: regexp.MustCompile(`(?:^|[^A-Za-z0-9])AKIA[0-9A-Z]{4,}`)},
		{label: "private key", re: regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
	}
)

type GlobalMemoryOption func(*GlobalMemoryStore)

func WithSoulOverridePath(path string) GlobalMemoryOption {
	return func(g *GlobalMemoryStore) {
		g.soulOverridePath = strings.TrimSpace(path)
		if g.soulOverridePath != "" {
			g.useConfigSoul = true
		}
	}
}

func WithUseConfigSoul(enabled bool) GlobalMemoryOption {
	return func(g *GlobalMemoryStore) {
		g.useConfigSoul = enabled
	}
}

func WithMemoryCharLimit(limit int) GlobalMemoryOption {
	return func(g *GlobalMemoryStore) {
		if limit > 0 {
			g.memoryLimit = limit
		}
	}
}

func WithUserCharLimit(limit int) GlobalMemoryOption {
	return func(g *GlobalMemoryStore) {
		if limit > 0 {
			g.userLimit = limit
		}
	}
}

type GlobalMemoryStore struct {
	mu               sync.Mutex
	dataDir          string
	memoryLimit      int
	userLimit        int
	soulOverridePath string
	useConfigSoul    bool
}

func NewGlobalMemoryStore(dataDir string, opts ...GlobalMemoryOption) *GlobalMemoryStore {
	g := &GlobalMemoryStore{
		dataDir:     dataDir,
		memoryLimit: DefaultMemoryCharLimit,
		userLimit:   DefaultUserCharLimit,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

func DefaultBaseIdentity() string {
	return strings.TrimSpace(`Match reply weight to the ask: answer simple requests directly and concisely; earn depth on complex tasks with concrete facts and clear structure.
Report only verified changes, actual check/command outcomes, and remaining work or blockers. Never fabricate tool output, file paths, or test results.
Skip filler, throat-clearing, and self-congratulation. Preserve existing user conventions and workspace boundaries.`)
}

func ScanMemoryContent(text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lower := strings.ToLower(text)
	for _, phrase := range injectionPhrases {
		if strings.Contains(lower, phrase) {
			return fmt.Errorf("memory content blocked: detected prompt-injection phrase %q", phrase)
		}
	}
	for _, pat := range secretPatterns {
		if pat.re.MatchString(text) {
			return fmt.Errorf("memory content blocked: detected potential secret (%s)", pat.label)
		}
	}
	return nil
}

func (g *GlobalMemoryStore) SetUseConfigSoul(enabled bool) {
	g.mu.Lock()
	g.useConfigSoul = enabled
	g.mu.Unlock()
}

func (g *GlobalMemoryStore) SetSoulOverridePath(path string) {
	g.mu.Lock()
	g.soulOverridePath = strings.TrimSpace(path)
	g.mu.Unlock()
}

func (g *GlobalMemoryStore) ApplyBatch(ctx context.Context, target string, ops []MemoryOp) (string, error) {
	if ctx != nil && ctx.Err() != nil {
		return "", ctx.Err()
	}

	fileName, limit, normTarget, err := g.resolveTarget(target)
	if err != nil {
		return "", err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	current, err := g.readFileIfExistsLocked(filepath.Join(g.dataDir, fileName))
	if err != nil {
		return "", err
	}

	entries := splitMemoryEntries(current)
	for i, op := range ops {
		action := strings.ToLower(strings.TrimSpace(op.Action))
		switch action {
		case "add":
			content := strings.TrimSpace(op.Content)
			if content == "" {
				return "", fmt.Errorf("op[%d] add requires non-empty content", i)
			}
			if err := ScanMemoryContent(content); err != nil {
				return "", err
			}
			entries = append(entries, content)
		case "patch":
			oldText := strings.TrimSpace(op.OldText)
			newText := strings.TrimSpace(op.NewText)
			if newText == "" {
				newText = strings.TrimSpace(op.Content)
			}
			if oldText == "" {
				return "", fmt.Errorf("op[%d] patch requires old_text", i)
			}
			if newText == "" {
				return "", fmt.Errorf("op[%d] patch requires new_text or content", i)
			}
			if err := ScanMemoryContent(newText); err != nil {
				return "", err
			}
			idx, matchErr := findMemoryEntryIndex(entries, oldText)
			if matchErr != nil {
				return "", fmt.Errorf("op[%d] patch: %w", i, matchErr)
			}
			if entries[idx] == oldText {
				entries[idx] = newText
			} else {
				entries[idx] = strings.TrimSpace(strings.Replace(entries[idx], oldText, newText, 1))
			}
			if entries[idx] == "" {
				entries = append(entries[:idx], entries[idx+1:]...)
			}
		case "remove":
			oldText := strings.TrimSpace(op.OldText)
			if oldText == "" {
				oldText = strings.TrimSpace(op.Content)
			}
			if oldText == "" {
				return "", fmt.Errorf("op[%d] remove requires old_text", i)
			}
			idx, matchErr := findMemoryEntryIndex(entries, oldText)
			if matchErr != nil {
				return "", fmt.Errorf("op[%d] remove: %w", i, matchErr)
			}
			entries = append(entries[:idx], entries[idx+1:]...)
		case "replace":
			content := strings.TrimSpace(op.Content)
			if content == "" {
				content = strings.TrimSpace(op.NewText)
			}
			if err := ScanMemoryContent(content); err != nil {
				return "", err
			}
			entries = splitMemoryEntries(content)
		default:
			return "", fmt.Errorf("op[%d] unsupported memory action %q", i, op.Action)
		}
	}

	finalContent := joinMemoryEntries(entries)
	if err := ScanMemoryContent(finalContent); err != nil {
		return "", err
	}

	charCount := utf8.RuneCountInString(finalContent)
	if limit > 0 && charCount > limit {
		return "", fmt.Errorf("%s exceeds character limit (%d/%d chars across %d entries); consolidate or remove entries", normTarget, charCount, limit, len(entries))
	}

	if err := g.writeFileAtomicLocked(filepath.Join(g.dataDir, fileName), finalContent); err != nil {
		return "", err
	}
	return finalContent, nil
}

func (g *GlobalMemoryStore) GetDoc(ctx context.Context) (GlobalMemoryDoc, error) {
	if ctx != nil && ctx.Err() != nil {
		return GlobalMemoryDoc{}, ctx.Err()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	mem, err := g.readFileIfExistsLocked(filepath.Join(g.dataDir, memoryFileName))
	if err != nil {
		return GlobalMemoryDoc{}, err
	}
	user, err := g.readFileIfExistsLocked(filepath.Join(g.dataDir, userFileName))
	if err != nil {
		return GlobalMemoryDoc{}, err
	}
	soul, err := g.readSoulLocked(false)
	if err != nil {
		return GlobalMemoryDoc{}, err
	}

	return GlobalMemoryDoc{
		Memory:      mem,
		UserProfile: user,
		Soul:        soul,
		MemoryLimit: g.memoryLimit,
		UserLimit:   g.userLimit,
		MemoryChars: utf8.RuneCountInString(mem),
		UserChars:   utf8.RuneCountInString(user),
	}, nil
}

func (g *GlobalMemoryStore) SaveDoc(ctx context.Context, doc GlobalMemoryDoc) (GlobalMemoryDoc, error) {
	if ctx != nil && ctx.Err() != nil {
		return GlobalMemoryDoc{}, ctx.Err()
	}

	if err := ScanMemoryContent(doc.Memory); err != nil {
		return GlobalMemoryDoc{}, err
	}
	if err := ScanMemoryContent(doc.UserProfile); err != nil {
		return GlobalMemoryDoc{}, err
	}

	memChars := utf8.RuneCountInString(doc.Memory)
	if g.memoryLimit > 0 && memChars > g.memoryLimit {
		entries := splitMemoryEntries(doc.Memory)
		return GlobalMemoryDoc{}, fmt.Errorf("memory exceeds character limit (%d/%d chars across %d entries)", memChars, g.memoryLimit, len(entries))
	}

	userChars := utf8.RuneCountInString(doc.UserProfile)
	if g.userLimit > 0 && userChars > g.userLimit {
		entries := splitMemoryEntries(doc.UserProfile)
		return GlobalMemoryDoc{}, fmt.Errorf("user profile exceeds character limit (%d/%d chars across %d entries)", userChars, g.userLimit, len(entries))
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.writeFileAtomicLocked(filepath.Join(g.dataDir, memoryFileName), doc.Memory); err != nil {
		return GlobalMemoryDoc{}, err
	}
	if err := g.writeFileAtomicLocked(filepath.Join(g.dataDir, userFileName), doc.UserProfile); err != nil {
		return GlobalMemoryDoc{}, err
	}

	soulPath := filepath.Join(g.dataDir, soulFileName)
	if strings.TrimSpace(doc.Soul) == "" {
		if err := os.Remove(soulPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return GlobalMemoryDoc{}, fmt.Errorf("remove %s: %w", soulPath, err)
		}
	} else {
		if err := g.writeFileAtomicLocked(soulPath, doc.Soul); err != nil {
			return GlobalMemoryDoc{}, err
		}
	}

	return GlobalMemoryDoc{
		Memory:      doc.Memory,
		UserProfile: doc.UserProfile,
		Soul:        doc.Soul,
		MemoryLimit: g.memoryLimit,
		UserLimit:   g.userLimit,
		MemoryChars: memChars,
		UserChars:   userChars,
	}, nil
}

func (g *GlobalMemoryStore) FormatFrozenSnapshot(ctx context.Context) (string, string) {
	if ctx != nil && ctx.Err() != nil {
		return "", DefaultBaseIdentity()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	mem, err := g.readFileIfExistsLocked(filepath.Join(g.dataDir, memoryFileName))
	if err != nil {
		mem = ""
	}
	user, err := g.readFileIfExistsLocked(filepath.Join(g.dataDir, userFileName))
	if err != nil {
		user = ""
	}
	soul, err := g.readSoulLocked(true)
	if err != nil || strings.TrimSpace(soul) == "" {
		soul = DefaultBaseIdentity()
	}

	memChars := utf8.RuneCountInString(mem)
	userChars := utf8.RuneCountInString(user)

	var b strings.Builder
	b.WriteString("<hermes_memory>\n")
	fmt.Fprintf(&b, "<memory chars=\"%d/%d\">\n", memChars, g.memoryLimit)
	if strings.TrimSpace(mem) != "" {
		b.WriteString(strings.TrimSpace(mem))
		b.WriteByte('\n')
	}
	b.WriteString("</memory>\n")
	fmt.Fprintf(&b, "<user_profile chars=\"%d/%d\">\n", userChars, g.userLimit)
	if strings.TrimSpace(user) != "" {
		b.WriteString(strings.TrimSpace(user))
		b.WriteByte('\n')
	}
	b.WriteString("</user_profile>\n")
	b.WriteString("</hermes_memory>")

	return b.String(), strings.TrimSpace(soul)
}

func (g *GlobalMemoryStore) resolveTarget(target string) (string, int, string, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "memory", "memory.md":
		return memoryFileName, g.memoryLimit, "memory", nil
	case "user", "user.md", "user_profile":
		return userFileName, g.userLimit, "user", nil
	default:
		return "", 0, "", fmt.Errorf("unsupported memory target %q: expected \"memory\" or \"user\"", target)
	}
}

func (g *GlobalMemoryStore) readSoulLocked(withDefault bool) (string, error) {
	localPath := filepath.Join(g.dataDir, soulFileName)
	data, err := os.ReadFile(localPath)
	if err == nil {
		return string(data), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read %s: %w", localPath, err)
	}

	if g.useConfigSoul {
		overridePath := g.soulOverridePath
		if overridePath == "" {
			overridePath = filepath.Join(config.GlobalConfigDir(), soulFileName)
		}
		if overrideData, overrideErr := os.ReadFile(overridePath); overrideErr == nil {
			return string(overrideData), nil
		} else if !errors.Is(overrideErr, os.ErrNotExist) {
			return "", fmt.Errorf("read %s: %w", overridePath, overrideErr)
		}
	}

	if withDefault {
		return DefaultBaseIdentity(), nil
	}
	return "", nil
}

func (g *GlobalMemoryStore) readFileIfExistsLocked(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

func (g *GlobalMemoryStore) writeFileAtomicLocked(targetPath, content string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create memory dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename %s: %w", targetPath, err)
	}
	return nil
}

func splitMemoryEntries(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return nil
	}
	parts := strings.Split(normalized, EntryDelimiter)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func joinMemoryEntries(entries []string) string {
	cleaned := make([]string, 0, len(entries))
	for _, e := range entries {
		trimmed := strings.TrimSpace(e)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, EntryDelimiter)
}

func findMemoryEntryIndex(entries []string, needle string) (int, error) {
	exactIdx := -1
	exactCount := 0
	for i, e := range entries {
		if e == needle {
			exactIdx = i
			exactCount++
		}
	}
	if exactCount == 1 {
		return exactIdx, nil
	}
	if exactCount > 1 {
		return -1, fmt.Errorf("target %q matched %d entries", needle, exactCount)
	}

	subIdx := -1
	subCount := 0
	for i, e := range entries {
		if strings.Contains(e, needle) {
			subIdx = i
			subCount++
		}
	}
	if subCount == 0 {
		return -1, fmt.Errorf("target %q not found", needle)
	}
	if subCount > 1 {
		return -1, fmt.Errorf("target %q matched %d entries; provide more specific old_text", needle, subCount)
	}
	return subIdx, nil
}
