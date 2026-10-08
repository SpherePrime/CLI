package instructions

import (
	"sort"
	"strings"
	"unicode"
)

func Search(query string, limit int) []Entry {
	if limit <= 0 {
		limit = 6
	}
	if limit > 12 {
		limit = 12
	}
	query = strings.ToLower(strings.TrimSpace(query))
	terms := strings.FieldsFunc(query, func(character rune) bool { return !unicode.IsLetter(character) && !unicode.IsDigit(character) })
	type rankedEntry struct {
		entry Entry
		score int
	}
	ranked := make([]rankedEntry, 0)
	for _, entry := range Catalog() {
		score := 0
		if query == "" {
			score = 1
		}
		fields := []string{entry.ID, entry.Title, entry.Description, entry.Category}
		fields = append(fields, entry.Keywords...)
		for _, term := range terms {
			best := 0
			for index, field := range fields {
				field = strings.ToLower(field)
				value := 0
				if field == term {
					value = 8
				} else if strings.Contains(field, term) {
					value = 2
				}
				if index == 0 && value > 0 {
					value += 4
				}
				if value > best {
					best = value
				}
			}
			score += best
		}
		if score > 0 {
			ranked = append(ranked, rankedEntry{entry, score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].entry.ID < ranked[j].entry.ID
		}
		return ranked[i].score > ranked[j].score
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	result := make([]Entry, 0, len(ranked))
	for _, match := range ranked {
		result = append(result, match.entry)
	}
	return result
}
