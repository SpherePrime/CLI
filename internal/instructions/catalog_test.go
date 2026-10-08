package instructions

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSourceSelectionHasReadableFullModules(t *testing.T) {
	data, err := bundledFiles.ReadFile("source-selection.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SourceCommit string `json:"source_commit"`
		Files        []struct {
			File, Status, Reason string
			Modules              []string
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SourceCommit != "9b3512f" || len(manifest.Files) != 870 {
		t.Fatal("incomplete source audit")
	}
	for _, file := range manifest.Files {
		if file.Reason == "" {
			t.Fatalf("missing reason: %s", file.File)
		}
		if file.Status != "selected" {
			continue
		}
		found := false
		for _, id := range file.Modules {
			if strings.HasPrefix(id, "source-") {
				if _, err := Read(id); err == nil {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("selected source lacks readable full adaptation: %s", file.File)
		}
	}
}

func TestSearchBilingualAndBounds(t *testing.T) {
	for _, query := range []string{"debugging", "отладка", "исправить ошибку"} {
		results := Search(query, 1)
		if len(results) != 1 || results[0].ID != "debugging" {
			t.Fatalf("%q: %#v", query, results)
		}
	}
	if len(Search("", 100)) != 12 || len(Search("", 0)) != 6 {
		t.Fatal("search limits")
	}
	if len(Search("nonexistent-zxq", 3)) != 0 {
		t.Fatal("unrelated results")
	}
	if !reflect.DeepEqual(Search("git", 12), Search("git", 12)) {
		t.Fatal("unstable ranking")
	}
}

func TestReferencesUsePrimeShellAndMCPContract(t *testing.T) {
	body, err := Read("source-bash-overview")
	if err != nil || !strings.Contains(body, "Bash-compatible") || strings.Contains(body, "may be PowerShell") {
		t.Fatal("incorrect Prime shell contract")
	}
	body, err = Read("source-listmcpresourcestool")
	if err != nil || !strings.Contains(body, `"mcp_name"`) || strings.Contains(body, "listMcpResources") {
		t.Fatal("incorrect MCP resource contract")
	}
}

func TestServerReferencesUseSupportedJobLifecycle(t *testing.T) {
	for _, id := range []string{"source-run-web-server-api-example", "source-verify-server-api-changes-example-for-verify-skill"} {
		body, err := Read(id)
		if err != nil { t.Fatal(err) }
		for _, stale := range []string{"curl", "$!", "SERVER_PID", "lsof", "npm start &"} {
			if strings.Contains(body, stale) { t.Fatalf("unsupported server recipe %q in %s", stale, id) }
		}
		for _, required := range []string{`"run_in_background": true`, `"shell_id"`, "job_output", "job_kill", "readiness", "deadline"} {
			if !strings.Contains(body, required) { t.Fatalf("missing lifecycle %q in %s", required, id) }
		}
	}
}

func TestReadIsolationAndUnknownIDs(t *testing.T) {
	body, err := Read("debugging")
	if err != nil || !strings.Contains(body, "reproduce") {
		t.Fatalf("debugging: %v %s", err, body)
	}
	for _, id := range []string{"../catalog.json", "library/debugging.md", "", "unknown"} {
		if _, err := Read(id); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	entries := Catalog()
	entries[0].Keywords[0] = "mutated"
	if Catalog()[0].Keywords[0] == "mutated" {
		t.Fatal("catalog mutation leaked")
	}
}

func TestCatalogBodiesAndProvenance(t *testing.T) {
	entries := Catalog()
	files, err := bundledFiles.ReadDir("library")
	if err != nil || len(files) != len(entries) {
		t.Fatal("orphaned or missing embedded module")
	}
	if len(entries) < 20 {
		t.Fatal("insufficient workflow coverage")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.ID] || entry.Description == "" || entry.Source == "" {
			t.Fatalf("invalid entry: %#v", entry)
		}
		seen[entry.ID] = true
		body, err := Read(entry.ID)
		if err != nil || len(body) < 100 || strings.Contains(body, "${") {
			t.Fatalf("invalid body %s: %v", entry.ID, err)
		}
		for _, forbidden := range []string{"Anthropic", "Claude", "EnterWorktree", "SuggestSkills"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("unadapted reference %s in %s", forbidden, entry.ID)
			}
		}
	}
}
