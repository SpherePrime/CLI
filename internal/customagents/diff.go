package customagents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type diffOp struct {
	kind byte
	line string
}

func UnifiedLineDiff(oldText, newText string, maxChars int) string {
	if oldText == newText {
		return ""
	}
	oldLines := splitNormalizedLines(oldText)
	newLines := splitNormalizedLines(newText)

	if len(oldLines)*len(newLines) > 400000 {
		return fallbackLineDiff(oldLines, newLines, maxChars)
	}

	ops := lcsDiffOps(oldLines, newLines)
	if len(ops) == 0 {
		return ""
	}

	var b strings.Builder
	const contextSize = 2
	n := len(ops)
	changed := make([]bool, n)
	hasChange := false
	for i, op := range ops {
		if op.kind != ' ' {
			hasChange = true
			start := max(0, i-contextSize)
			end := min(n-1, i+contextSize)
			for j := start; j <= end; j++ {
				changed[j] = true
			}
		}
	}
	if !hasChange {
		return ""
	}

	inHunk := false
	for i, op := range ops {
		if !changed[i] {
			if inHunk {
				inHunk = false
			}
			continue
		}
		if !inHunk {
			if b.Len() > 0 {
				b.WriteString("...\n")
			}
			inHunk = true
		}
		b.WriteByte(op.kind)
		b.WriteString(op.line)
		b.WriteByte('\n')
		if maxChars > 0 && b.Len() >= maxChars {
			break
		}
	}

	out := strings.TrimRight(b.String(), "\n")
	if maxChars > 0 && utf8.RuneCountInString(out) > maxChars {
		return truncateUTF8(out, maxChars, "\n... (truncated)")
	}
	return out
}

func truncateUTF8(s string, maxRunes int, suffix string) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	suffixRunes := utf8.RuneCountInString(suffix)
	keep := maxRunes
	if suffix != "" && maxRunes > suffixRunes {
		keep = maxRunes - suffixRunes
	} else if suffix != "" {
		suffix = ""
	}
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= keep {
			break
		}
		b.WriteRune(r)
		count++
	}
	b.WriteString(suffix)
	return b.String()
}

func splitNormalizedLines(s string) []string {
	if s == "" {
		return nil
	}
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(normalized, "\n")
}

func lcsDiffOps(a, b []string) []diffOp {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	ops := make([]diffOp, 0, m+n)
	i, j := 0, 0
	for i < m && j < n {
		if a[i] == b[j] {
			ops = append(ops, diffOp{kind: ' ', line: a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOp{kind: '-', line: a[i]})
			i++
		} else {
			ops = append(ops, diffOp{kind: '+', line: b[j]})
			j++
		}
	}
	for i < m {
		ops = append(ops, diffOp{kind: '-', line: a[i]})
		i++
	}
	for j < n {
		ops = append(ops, diffOp{kind: '+', line: b[j]})
		j++
	}
	return ops
}

func fallbackLineDiff(oldLines, newLines []string, maxChars int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- old (%d lines)\n+++ new (%d lines)\n", len(oldLines), len(newLines))
	limit := min(len(oldLines), len(newLines))
	shown := 0
	for i := range limit {
		if oldLines[i] != newLines[i] {
			fmt.Fprintf(&b, "-%s\n+%s\n", oldLines[i], newLines[i])
			shown++
			if shown >= 40 || (maxChars > 0 && b.Len() >= maxChars) {
				break
			}
		}
	}
	if len(oldLines) > limit {
		for i := limit; i < len(oldLines) && shown < 40; i++ {
			fmt.Fprintf(&b, "-%s\n", oldLines[i])
			shown++
		}
	}
	if len(newLines) > limit {
		for i := limit; i < len(newLines) && shown < 40; i++ {
			fmt.Fprintf(&b, "+%s\n", newLines[i])
			shown++
		}
	}
	out := strings.TrimRight(b.String(), "\n")
	if maxChars > 0 && utf8.RuneCountInString(out) > maxChars {
		return truncateUTF8(out, maxChars, "\n... (truncated)")
	}
	return out
}
