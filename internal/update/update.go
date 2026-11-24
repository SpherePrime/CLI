package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	githubApiUrl = "https://api.github.com/repos/SpherePrime/CLI/releases/latest"
	userAgent    = "prime/1.0"
)

// Default is the default [Client].
var Default Client = &github{}

type Info struct {
	Current string
	Latest  string
	URL     string
	release *Release
}

var goInstallRegexp = regexp.MustCompile(`^v?\d+\.\d+\.\d+-\d+\.\d{14}-[0-9a-f]{12}$`)

func (i Info) IsDevelopment() bool {
	return i.Current == "devel" || i.Current == "unknown" || strings.Contains(i.Current, "dirty") || goInstallRegexp.MatchString(i.Current)
}

type parsedSemver struct {
	major int
	minor int
	patch int
	pre   []string
}

func parseSemver(raw string) (parsedSemver, bool) {
	v := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	if v == "" {
		return parsedSemver{}, false
	}
	if core, _, ok := strings.Cut(v, "+"); ok {
		v = core
	}
	core, preStr, hasPre := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return parsedSemver{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return parsedSemver{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return parsedSemver{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return parsedSemver{}, false
	}
	var pre []string
	if hasPre {
		if preStr == "" {
			return parsedSemver{}, false
		}
		pre = strings.Split(preStr, ".")
	}
	return parsedSemver{major: major, minor: minor, patch: patch, pre: pre}, true
}

func comparePrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	n := min(len(a), len(b))
	for i := range n {
		if a[i] == b[i] {
			continue
		}
		an, aErr := strconv.Atoi(a[i])
		bn, bErr := strconv.Atoi(b[i])
		switch {
		case aErr == nil && bErr == nil:
			if an < bn {
				return -1
			}
			return 1
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		case a[i] < b[i]:
			return -1
		default:
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func compareSemver(a, b parsedSemver) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}
	return comparePrerelease(a.pre, b.pre)
}

func (i Info) Available() bool {
	if i.IsDevelopment() {
		return false
	}
	current, okC := parseSemver(i.Current)
	latest, okL := parseSemver(i.Latest)
	if !okC || !okL {
		return false
	}
	if len(latest.pre) > 0 && len(current.pre) == 0 {
		return false
	}
	return compareSemver(latest, current) > 0
}

func Check(ctx context.Context, current string, client Client) (Info, error) {
	info := Info{
		Current: current,
		Latest:  current,
	}

	release, err := client.Latest(ctx)
	if err != nil {
		return info, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.Current = strings.TrimPrefix(info.Current, "v")
	info.URL = release.HTMLURL
	info.release = release
	return info, nil
}

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset is one file attached to a [Release].
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Client is a client that can get the latest release.
type Client interface {
	Latest(ctx context.Context) (*Release, error)
}

type github struct{}

// Latest implements [Client].
func (c *github) Latest(ctx context.Context) (*Release, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubApiUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
