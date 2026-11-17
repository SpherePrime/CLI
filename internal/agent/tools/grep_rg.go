package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ripgrepMatch struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
		Submatches []struct {
			Start int `json:"start"`
		} `json:"submatches"`
	} `json:"data"`
}

func searchWithRipgrep(ctx context.Context, pattern, path, include string) ([]grepMatch, error) {
	return searchWithRipgrepWithLimit(ctx, pattern, path, include, 100)
}

func searchWithRipgrepWithLimit(ctx context.Context, pattern, path, include string, limit int) ([]grepMatch, error) {
	rgCtx, cancelRg := context.WithCancel(ctx)
	defer cancelRg()

	cmd := getRgSearchCmd(rgCtx, pattern, path, include)
	if cmd == nil {
		return nil, fmt.Errorf("ripgrep not found in $PATH")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ripgrep: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ripgrep: %w", err)
	}

	candidatePool := max(limit*10, 500)

	type statResult struct {
		modTime time.Time
		ok      bool
	}
	statCache := make(map[string]statResult)

	var matches []grepMatch
	reader := bufio.NewReaderSize(stdout, 64*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			var match ripgrepMatch
			if json.Unmarshal(line, &match) == nil && match.Type == "match" && len(match.Data.Submatches) > 0 {
				filePath := match.Data.Path.Text
				st, cached := statCache[filePath]
				if !cached {
					if fi, statErr := os.Stat(filePath); statErr == nil {
						st = statResult{modTime: fi.ModTime(), ok: true}
					}
					statCache[filePath] = st
				}
				if st.ok {
					matches = append(matches, grepMatch{
						path:     filePath,
						modTime:  st.modTime,
						lineNum:  match.Data.LineNumber,
						charNum:  match.Data.Submatches[0].Start + 1,
						lineText: truncateGrepLine(strings.TrimSpace(match.Data.Lines.Text)),
					})
					if len(matches) >= candidatePool {
						break
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	earlyStopped := len(matches) >= candidatePool
	if earlyStopped {
		cancelRg()
	}
	_ = stdout.Close()
	waitErr := cmd.Wait()
	if earlyStopped && ctx.Err() == nil {
		return matches, nil
	}
	if waitErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 1 {
			return []grepMatch{}, nil
		}
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ripgrep: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		return nil, waitErr
	}

	return matches, nil
}
