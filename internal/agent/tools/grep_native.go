package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/vendordeps/bmatcuk/doublestar/v4"
	"github.com/SpherePrime/CLI/vendordeps/charlievieth/fastwalk"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
)

const (
	maxGrepFileSize = 2 * 1024 * 1024
	maxPoolBufCap   = 512 * 1024
)

var grepBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 32*1024)
		return &buf
	},
}

var binaryExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".o": true, ".obj": true, ".lib": true, ".bin": true, ".class": true,
	".pyc": true, ".pyo": true, ".wasm": true, ".pdb": true, ".ilk": true,
	".exp": true, ".zip": true, ".tar": true, ".gz": true, ".tgz": true,
	".bz2": true, ".xz": true, ".7z": true, ".rar": true, ".zst": true,
	".lz4": true, ".cab": true, ".iso": true, ".dmg": true, ".pkg": true,
	".deb": true, ".rpm": true, ".msi": true, ".jar": true, ".war": true,
	".ear": true, ".apk": true, ".ipa": true, ".png": true, ".jpg": true,
	".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".icns": true,
	".webp": true, ".avif": true, ".tiff": true, ".tif": true, ".psd": true,
	".raw": true, ".heic": true, ".heif": true, ".mp3": true, ".mp4": true,
	".wav": true, ".flac": true, ".ogg": true, ".aac": true, ".m4a": true,
	".wma": true, ".avi": true, ".mkv": true, ".mov": true, ".wmv": true,
	".webm": true, ".flv": true, ".m4v": true, ".ttf": true, ".otf": true,
	".woff": true, ".woff2": true, ".eot": true, ".pdf": true, ".doc": true,
	".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".odt": true, ".ods": true, ".odp": true, ".db": true, ".sqlite": true,
	".sqlite3": true, ".mdb": true, ".parquet": true, ".avro": true,
	".arrow": true, ".feather": true, ".lockb": true, ".pack": true,
	".idx": true, ".ds_store": true,
}

type lineMatch struct {
	lineNum  int
	charNum  int
	lineText string
}

func searchFilesWithRegex(pattern, rootPath, include string) ([]grepMatch, error) {
	return searchFilesWithRegexCtx(context.Background(), pattern, rootPath, include, 100)
}

func searchFilesWithRegexCtx(ctx context.Context, pattern, rootPath, include string, limit int) ([]grepMatch, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	regex, err := searchRegexCache.get(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	includePattern := normalizeIncludePattern(include)
	if includePattern != "" && !doublestar.ValidatePattern(includePattern) {
		return nil, fmt.Errorf("invalid include pattern: %s", include)
	}

	if rootInfo, statErr := os.Stat(rootPath); statErr == nil && !rootInfo.IsDir() {
		return searchSingleTargetFile(rootPath, rootInfo, includePattern, regex)
	}

	candidateCap := int64(max(limit*5, 500))

	if entries, ok := fsext.IndexEntries(ctx, rootPath); ok {
		return searchIndexedEntries(ctx, rootPath, entries, includePattern, regex, candidateCap)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return searchWithFastwalk(ctx, rootPath, includePattern, regex, candidateCap)
}

func searchSingleTargetFile(filePath string, info os.FileInfo, includePattern string, regex *regexp.Regexp) ([]grepMatch, error) {
	base := filepath.Base(filePath)
	if base != "." && strings.HasPrefix(base, ".") {
		return nil, nil
	}
	if isBinaryExtension(base) || info.Size() > maxGrepFileSize {
		return nil, nil
	}
	if !matchesInclude(includePattern, filepath.ToSlash(base), base) {
		return nil, nil
	}
	lineMatches, modTime, err := searchSingleFile(filePath, info.ModTime(), regex)
	if err != nil {
		return nil, nil
	}
	matches := make([]grepMatch, 0, len(lineMatches))
	for _, lm := range lineMatches {
		matches = append(matches, grepMatch{
			path:     filePath,
			modTime:  modTime,
			lineNum:  lm.lineNum,
			charNum:  lm.charNum,
			lineText: lm.lineText,
		})
	}
	return matches, nil
}

func searchIndexedEntries(
	ctx context.Context,
	rootPath string,
	entries []fsext.IndexEntry,
	includePattern string,
	regex *regexp.Regexp,
	candidateCap int64,
) ([]grepMatch, error) {
	candidates := make([]fsext.IndexEntry, 0, min(len(entries), 1024))
	for i := range entries {
		entry := entries[i]
		if entry.IsDir || entry.Size > maxGrepFileSize {
			continue
		}
		if hasHiddenSegment(entry.SlashPath) || isBinaryExtension(entry.Name) {
			continue
		}
		if !matchesInclude(includePattern, entry.SlashPath, entry.Name) {
			continue
		}
		candidates = append(candidates, entry)
	}

	if len(candidates) == 0 {
		return []grepMatch{}, nil
	}

	slices.SortFunc(candidates, func(a, b fsext.IndexEntry) int {
		return b.ModTime.Compare(a.ModTime)
	})

	workerCount := min(max(runtime.GOMAXPROCS(0), 1), len(candidates), 16)
	var (
		nextIdx    atomic.Int64
		matchCount atomic.Int64
		mu         sync.Mutex
		wg         sync.WaitGroup
		matches    []grepMatch
	)

	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx.Err() != nil || matchCount.Load() >= candidateCap {
					return
				}
				idx := int(nextIdx.Add(1) - 1)
				if idx >= len(candidates) {
					return
				}
				entry := candidates[idx]
				matchPath := filepath.Join(rootPath, entry.RelPath)
				lineMatches, modTime, err := searchSingleFile(matchPath, entry.ModTime, regex)
				if err != nil || len(lineMatches) == 0 {
					continue
				}
				mu.Lock()
				for _, lm := range lineMatches {
					matches = append(matches, grepMatch{
						path:     matchPath,
						modTime:  modTime,
						lineNum:  lm.lineNum,
						charNum:  lm.charNum,
						lineText: lm.lineText,
					})
				}
				mu.Unlock()
				matchCount.Add(int64(len(lineMatches)))
			}
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return matches, nil
}

func searchWithFastwalk(
	ctx context.Context,
	rootPath string,
	includePattern string,
	regex *regexp.Regexp,
	candidateCap int64,
) ([]grepMatch, error) {
	walker := fsext.NewFastGlobWalker(rootPath)
	cleanRoot := filepath.Clean(rootPath)

	var (
		matchCount atomic.Int64
		mu         sync.Mutex
		matches    []grepMatch
	)

	conf := fastwalk.Config{
		Follow:  false,
		ToSlash: fastwalk.DefaultToSlash(),
		Sort:    fastwalk.SortFilesFirst,
	}

	err := fastwalk.Walk(&conf, rootPath, func(path string, d os.DirEntry, walkErr error) error {
		if ctx.Err() != nil || matchCount.Load() >= candidateCap {
			return filepath.SkipAll
		}
		if walkErr != nil {
			return nil
		}

		base := d.Name()
		if d.IsDir() {
			if path != rootPath && base != "." && strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			if walker.ShouldSkipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if base != "." && strings.HasPrefix(base, ".") {
			return nil
		}
		if walker.ShouldSkip(path) || isBinaryExtension(base) {
			return nil
		}
		if includePattern != "" {
			slashRel := fastSlashRelPath(cleanRoot, path)
			if !matchesInclude(includePattern, slashRel, base) {
				return nil
			}
		}

		var knownModTime time.Time
		if d.Type().IsRegular() {
			info, infoErr := d.Info()
			if infoErr != nil || info.Size() > maxGrepFileSize {
				return nil
			}
			knownModTime = info.ModTime()
		}

		lineMatches, modTime, searchErr := searchSingleFile(path, knownModTime, regex)
		if searchErr != nil || len(lineMatches) == 0 {
			return nil
		}

		mu.Lock()
		for _, lm := range lineMatches {
			matches = append(matches, grepMatch{
				path:     path,
				modTime:  modTime,
				lineNum:  lm.lineNum,
				charNum:  lm.charNum,
				lineText: lm.lineText,
			})
		}
		mu.Unlock()

		if matchCount.Add(int64(len(lineMatches))) >= candidateCap {
			return filepath.SkipAll
		}
		return nil
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return nil, err
	}
	return matches, nil
}

func searchSingleFile(filePath string, knownModTime time.Time, pattern *regexp.Regexp) ([]lineMatch, time.Time, error) {
	if pattern == nil {
		return nil, time.Time{}, nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer file.Close()

	modTime := knownModTime
	var expectedSize int64 = -1
	if modTime.IsZero() {
		fi, statErr := file.Stat()
		if statErr != nil {
			return nil, time.Time{}, statErr
		}
		if fi.IsDir() || fi.Size() > maxGrepFileSize {
			return nil, time.Time{}, nil
		}
		modTime = fi.ModTime()
		expectedSize = fi.Size()
	}

	bufPtr := grepBufPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	if expectedSize > 0 && int64(cap(buf)) < expectedSize+1 && expectedSize <= maxGrepFileSize {
		buf = make([]byte, 0, expectedSize+1)
	}
	defer func() {
		if cap(buf) <= maxPoolBufCap {
			*bufPtr = buf[:0]
			grepBufPool.Put(bufPtr)
		}
	}()

	data, overLimit, err := readBounded(file, buf, maxGrepFileSize)
	buf = data
	if err != nil || overLimit || len(data) == 0 {
		return nil, modTime, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, modTime, nil
	}

	var matches []lineMatch
	lineNum := 0
	rem := data
	for len(rem) > 0 {
		lineNum++
		var line []byte
		if idx := bytes.IndexByte(rem, '\n'); idx >= 0 {
			line = rem[:idx]
			rem = rem[idx+1:]
		} else {
			line = rem
			rem = nil
		}
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if loc := pattern.FindIndex(line); loc != nil {
			matches = append(matches, lineMatch{
				lineNum:  lineNum,
				charNum:  loc[0] + 1,
				lineText: formatGrepBytes(line),
			})
		}
	}
	return matches, modTime, nil
}

func readBounded(r io.Reader, buf []byte, maxBytes int64) ([]byte, bool, error) {
	for {
		if int64(len(buf)) > maxBytes {
			return buf, true, nil
		}
		if len(buf) == cap(buf) {
			nextCap := max(cap(buf)*2, 4096)
			if int64(nextCap) > maxBytes+1 {
				nextCap = int(maxBytes) + 1
			}
			grown := make([]byte, len(buf), nextCap)
			copy(grown, buf)
			buf = grown
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if int64(len(buf)) > maxBytes {
			return buf, true, nil
		}
		if err == io.EOF {
			return buf, false, nil
		}
		if err != nil {
			return buf, false, err
		}
	}
}

func formatGrepBytes(line []byte) string {
	if len(line) <= maxGrepContentWidth {
		return string(line)
	}
	if len(line) > maxGrepContentWidth*4 {
		cutoff := maxGrepContentWidth * 4
		for cutoff > 0 && !utf8.RuneStart(line[cutoff]) {
			cutoff--
		}
		line = line[:cutoff]
	}
	s := string(line)
	if ansi.StringWidth(s) > maxGrepContentWidth {
		return ansi.Truncate(s, maxGrepContentWidth, "...")
	}
	return s
}

func isBinaryExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExtensions[ext]
}

func isTextFile(filePath string) bool {
	if isBinaryExtension(filePath) {
		return false
	}
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	var buf [8192]byte
	n, err := file.Read(buf[:])
	if err != nil && err != io.EOF {
		return false
	}
	return bytes.IndexByte(buf[:n], 0) < 0
}

func normalizeIncludePattern(include string) string {
	if include == "" {
		return ""
	}
	return strings.TrimPrefix(filepath.ToSlash(include), "./")
}

func matchesInclude(includePattern, slashRelPath, baseName string) bool {
	if includePattern == "" {
		return true
	}
	if matched, err := doublestar.Match(includePattern, slashRelPath); err == nil && matched {
		return true
	}
	if !strings.Contains(includePattern, "/") {
		if matched, err := doublestar.Match(includePattern, baseName); err == nil && matched {
			return true
		}
	}
	return false
}

func hasHiddenSegment(slashPath string) bool {
	return strings.HasPrefix(slashPath, ".") || strings.Contains(slashPath, "/.")
}

func fastSlashRelPath(cleanRoot, path string) string {
	if cleanRoot != "." {
		prefix := cleanRoot + string(filepath.Separator)
		if strings.HasPrefix(path, prefix) {
			return filepath.ToSlash(path[len(prefix):])
		}
	}
	rel, err := filepath.Rel(cleanRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
