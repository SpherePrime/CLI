package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
)

const projectsFileName = "projects.json"

// Project represents a tracked project directory.
type Project struct {
	Path         string    `json:"path"`
	DataDir      string    `json:"data_dir"`
	LastAccessed time.Time `json:"last_accessed"`
}

// ProjectList holds the list of tracked projects.
type ProjectList struct {
	Projects []Project `json:"projects"`
}

var mu sync.Mutex

// projectsFilePath returns the path to the projects.json file.
func projectsFilePath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), projectsFileName)
}

// Load reads the projects list from disk.
func Load() (*ProjectList, error) {
	mu.Lock()
	defer mu.Unlock()

	path := projectsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectList{Projects: []Project{}}, nil
		}
		return nil, err
	}

	var list ProjectList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

// Save writes the projects list to disk.
func Save(list *ProjectList) error {
	mu.Lock()
	defer mu.Unlock()

	path := projectsFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// Register adds or updates a project in the list.
func Register(workingDir, dataDir string) error {
	list, err := Load()
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// Check if project already exists
	found := false
	for i, p := range list.Projects {
		if p.Path == workingDir {
			list.Projects[i].DataDir = dataDir
			list.Projects[i].LastAccessed = now
			found = true
			break
		}
	}

	if !found {
		list.Projects = append(list.Projects, Project{
			Path:         workingDir,
			DataDir:      dataDir,
			LastAccessed: now,
		})
	}

	// Sort by last accessed (most recent first).
	sortMostRecentFirst(list.Projects, workingDir)

	return Save(list)
}

// sortMostRecentFirst orders projects by last accessed, newest first, and puts
// mostRecent at the front even when timestamps tie.
//
// Two things make the tie the common case rather than the exotic one. The
// system clock is coarse on Windows, so registering twice inside one tick
// gives both projects the same LastAccessed; and time.Now().UTC() strips the
// monotonic reading, so After and Before compare wall clocks that can repeat.
// slices.SortFunc is not stable either, so without an explicit rule the
// project the user just opened could sort behind an older one and the order
// could change from run to run.
//
// Paths break the remaining ties to keep the sort reproducible; the project
// that was just registered or opened is then moved to the front, which is
// what "most recent" is supposed to mean.
func sortMostRecentFirst(projects []Project, mostRecent string) {
	slices.SortFunc(projects, func(a, b Project) int {
		if a.LastAccessed.After(b.LastAccessed) {
			return -1
		}
		if a.LastAccessed.Before(b.LastAccessed) {
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})

	if i := slices.IndexFunc(projects, func(p Project) bool { return p.Path == mostRecent }); i > 0 {
		// Rotate the entry and everything above it down one slot. The slices
		// package has no Move, and this keeps the relative order of the rest.
		entry := projects[i]
		copy(projects[1:i+1], projects[:i])
		projects[0] = entry
	}
}

// List returns all tracked projects sorted by last accessed.
func List() ([]Project, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	return list.Projects, nil
}

// Find resolves a tracked project by its path or one-based list index.
func Find(selector string) (Project, error) {
	list, err := Load()
	if err != nil {
		return Project{}, err
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return Project{}, os.ErrInvalid
	}
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 1 || index > len(list.Projects) {
			return Project{}, os.ErrNotExist
		}
		return list.Projects[index-1], nil
	}
	path, err := filepath.Abs(selector)
	if err != nil {
		return Project{}, err
	}
	for _, project := range list.Projects {
		projectPath, absErr := filepath.Abs(project.Path)
		if project.Path == selector || absErr == nil && projectPath == path {
			return project, nil
		}
	}
	return Project{}, os.ErrNotExist
}

// Prune removes tracked projects whose directories no longer exist.
func Prune() error {
	list, err := Load()
	if err != nil {
		return err
	}

	kept := make([]Project, 0, len(list.Projects))
	for _, project := range list.Projects {
		info, err := os.Stat(project.Path)
		if err == nil && info.IsDir() {
			kept = append(kept, project)
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if len(kept) == len(list.Projects) {
		return nil
	}
	list.Projects = kept
	return Save(list)
}

// Remove deletes a project from the tracked list without touching its directory.
func Remove(path string) error {
	list, err := Load()
	if err != nil {
		return err
	}
	kept := list.Projects[:0]
	for _, project := range list.Projects {
		if project.Path != path {
			kept = append(kept, project)
		}
	}
	list.Projects = kept
	return Save(list)
}

// Touch marks a tracked project as the most recently used project.
func Touch(path string) error {
	project, err := Find(path)
	if err != nil {
		return err
	}
	return Register(project.Path, project.DataDir)
}
