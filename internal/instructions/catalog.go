package instructions

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed catalog.json library/*.md LICENSE source-selection.json
var bundledFiles embed.FS

type Entry struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Source      string   `json:"source"`
	Keywords    []string `json:"keywords"`
}

var entries = loadCatalog()

func loadCatalog() []Entry {
	data, err := bundledFiles.ReadFile("catalog.json")
	if err != nil {
		panic(err)
	}
	var result []Entry
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func Catalog() []Entry {
	result := make([]Entry, len(entries))
	for index, entry := range entries {
		result[index] = entry
		result[index].Keywords = append([]string(nil), entry.Keywords...)
	}
	return result
}

func Read(id string) (string, error) {
	for _, entry := range entries {
		if entry.ID == id {
			body, err := bundledFiles.ReadFile("library/" + entry.ID + ".md")
			return string(body), err
		}
	}
	return "", fmt.Errorf("unknown instruction ID %q; use search_instructions to discover available IDs", id)
}

func Categories() []string {
	unique := map[string]bool{}
	for _, entry := range entries {
		unique[entry.Category] = true
	}
	result := make([]string, 0, len(unique))
	for category := range unique {
		result = append(result, category)
	}
	sort.Strings(result)
	return result
}
