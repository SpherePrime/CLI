package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/question"
)

const questionShapeHint = `Each question is one object with its options nested inside it, e.g. {"type":"single_choice","question":"Which drink?","description":"Pick one.","choices":[{"id":"tea","label":"Tea"},{"id":"coffee","label":"Coffee"}]}`

// decodeFirstJSONValue decodes the first JSON value in s and ignores any
// trailing characters some models append after a string-encoded array.
func decodeFirstJSONValue(s string, out any) error {
	return json.NewDecoder(strings.NewReader(strings.TrimSpace(s))).Decode(out)
}

// UnmarshalJSON accepts a choice as an object or as a plain string label, and
// tolerates common alternative key names.
func (c *QuestionChoice) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var label string
		if err := json.Unmarshal(trimmed, &label); err != nil {
			return err
		}
		*c = QuestionChoice{Label: label}
		return nil
	}
	var raw struct {
		ID          string `json:"id"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Value       string `json:"value"`
		Text        string `json:"text"`
		Title       string `json:"title"`
		Name        string `json:"name"`
	}
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return err
	}
	*c = QuestionChoice{
		ID:          firstNonEmpty(raw.ID, raw.Value),
		Label:       firstNonEmpty(raw.Label, raw.Text, raw.Title, raw.Name, raw.Value),
		Description: raw.Description,
	}
	return nil
}

// UnmarshalJSON accepts alternative key names models use for the question
// text and its options.
func (q *QuestionItem) UnmarshalJSON(data []byte) error {
	type plain QuestionItem
	aux := struct {
		*plain
		ID       string           `json:"id"`
		Text     string           `json:"text"`
		Title    string           `json:"title"`
		Answers  []QuestionChoice `json:"answers"`
		Variants []QuestionChoice `json:"variants"`
	}{plain: (*plain)(q)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	q.id = aux.ID
	if q.Question == "" {
		q.Question = firstNonEmpty(aux.Text, aux.Title)
	}
	for _, alternative := range [][]QuestionChoice{q.Options, aux.Answers, aux.Variants} {
		if len(q.Choices) == 0 {
			q.Choices = alternative
		}
	}
	return nil
}

// normalizeQuestionItems repairs common malformed shapes: options sent as
// separate items after their question, a missing question text or type, and
// choices without ids.
func normalizeQuestionItems(items []QuestionItem) ([]QuestionItem, error) {
	normalized := make([]QuestionItem, 0, len(items))
	for _, item := range items {
		if looksLikeChoice(item) {
			if last := len(normalized) - 1; last >= 0 && isChoiceType(normalized[last].Type) {
				normalized[last].Choices = append(normalized[last].Choices, QuestionChoice{
					ID:          item.id,
					Label:       firstNonEmpty(item.Label, item.id),
					Description: item.Description,
				})
				continue
			}
			return nil, fmt.Errorf("question %d [%s] looks like an answer option, not a question. %s", len(normalized)+1, firstNonEmpty(item.Label, item.id), questionShapeHint)
		}
		if item.Question == "" {
			item.Question = item.Label
		}
		if item.Type == "" {
			if len(item.Choices) >= 2 {
				item.Type = string(question.TypeSingleChoice)
			} else {
				item.Type = string(question.TypeFreeText)
			}
		}
		item.Choices = withChoiceIDs(item.Choices)
		normalized = append(normalized, item)
	}
	for i, item := range normalized {
		if isChoiceType(item.Type) && len(item.Choices) < 2 {
			return nil, fmt.Errorf("question %d [%s]: %s needs at least 2 options in \"choices\" (got %d). %s", i+1, firstNonEmpty(item.Label, item.Question), item.Type, len(item.Choices), questionShapeHint)
		}
	}
	return normalized, nil
}

func looksLikeChoice(item QuestionItem) bool {
	if item.Type == string(question.TypeFreeText) || item.Type == string(question.TypeYesNo) {
		return false
	}
	return item.id != "" && item.Question == "" && len(item.Choices) == 0
}

func isChoiceType(value string) bool {
	return value == string(question.TypeSingleChoice) || value == string(question.TypeMultiChoice)
}

func withChoiceIDs(choices []QuestionChoice) []QuestionChoice {
	if len(choices) == 0 {
		return choices
	}
	out := make([]QuestionChoice, len(choices))
	seen := make(map[string]bool, len(choices))
	for i, choice := range choices {
		if choice.ID == "" {
			choice.ID = choice.Label
		}
		if choice.Label == "" {
			choice.Label = choice.ID
		}
		if choice.ID == "" || seen[choice.ID] {
			choice.ID = fmt.Sprintf("choice-%d", i+1)
		}
		seen[choice.ID] = true
		out[i] = choice
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
