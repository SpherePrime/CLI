package tools

import (
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/internal/question"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestQuestionParamsUnmarshalJSON_NativeArray(t *testing.T) {
	t.Parallel()
	input := `{"questions": [{"type": "yes_no", "question": "OK?", "description": "test"}]}`
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	require.Len(t, p.Questions, 1)
	require.Equal(t, "OK?", p.Questions[0].Question)
}

func TestQuestionParamsUnmarshalJSON_StringEncodedArray(t *testing.T) {
	t.Parallel()
	// Simulates a model that double-serializes the questions field.
	inner := `[{"type":"yes_no","question":"OK?","description":"test"}]`
	encoded, _ := json.Marshal(inner)
	input := `{"questions": ` + string(encoded) + `}`
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	require.Len(t, p.Questions, 1)
	require.Equal(t, "OK?", p.Questions[0].Question)
}

func TestQuestionParamsUnmarshalJSON_StringEncodedWithWhitespace(t *testing.T) {
	t.Parallel()
	inner := `  [{"type":"single_choice","question":"Pick","description":"d","choices":[{"id":"a","label":"A"}]}]  `
	encoded, _ := json.Marshal(inner)
	input := `{"questions": ` + string(encoded) + `, "confirm_title": "Go?"}`
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	require.Len(t, p.Questions, 1)
	require.Equal(t, "Pick", p.Questions[0].Question)
	require.Equal(t, "Go?", p.ConfirmTitle)
}

func TestQuestionParamsUnmarshalJSON_InvalidString(t *testing.T) {
	t.Parallel()
	encoded, _ := json.Marshal("not valid json")
	input := `{"questions": ` + string(encoded) + `}`
	var p QuestionParams
	require.Error(t, json.Unmarshal([]byte(input), &p))
}

func TestFormatAnswer_MultiChoiceWithFillIn(t *testing.T) {
	answer := question.Answer{
		SelectedIDs: []string{"speed", "readability"},
		FillInText:  "maintainability",
	}
	resp, err := formatAnswer(&answer, question.TypeMultiChoice)
	require.NoError(t, err)
	require.Contains(t, resp.Content, `User selected: ["speed","readability"]`)
	require.Contains(t, resp.Content, "User provided: maintainability")
}

func TestFormatAnswer_SelectionsOnly(t *testing.T) {
	answer := question.Answer{SelectedIDs: []string{"gardening"}}
	resp, err := formatAnswer(&answer, question.TypeSingleChoice)
	require.NoError(t, err)
	require.Contains(t, resp.Content, `User selected: ["gardening"]`)
	require.NotContains(t, resp.Content, "User provided")
}

func TestFormatAnswer_Skipped(t *testing.T) {
	answer := question.Answer{}
	resp, err := formatAnswer(&answer, question.TypeFreeText)
	require.NoError(t, err)
	require.Equal(t, "User skipped this question", resp.Content)
}

func normalizedInput(t *testing.T, input string) ([]QuestionItem, error) {
	t.Helper()
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	return normalizeQuestionItems(p.Questions)
}

func TestQuestionInput_StringChoicesAndOptionsAlias(t *testing.T) {
	t.Parallel()
	items, err := normalizedInput(t, `{"questions":[{"label":"Напиток","type":"single_choice","question":"Что пьём?","description":"d","options":["Чай","Кофе"]}]}`)
	require.NoError(t, err)
	require.Equal(t, []QuestionChoice{{ID: "Чай", Label: "Чай"}, {ID: "Кофе", Label: "Кофе"}}, items[0].Choices)
}

func TestQuestionInput_AlternativeChoiceKeys(t *testing.T) {
	t.Parallel()
	items, err := normalizedInput(t, `{"questions":[{"type":"multi_choice","text":"Силы?","description":"d","variants":[{"value":"fly","text":"Полёт"},{"value":"fly","name":"Невидимость"}]}]}`)
	require.NoError(t, err)
	require.Equal(t, "Силы?", items[0].Question)
	require.Equal(t, []QuestionChoice{{ID: "fly", Label: "Полёт"}, {ID: "choice-2", Label: "Невидимость"}}, items[0].Choices)
}

func TestQuestionInput_OptionsSentAsSeparateItems(t *testing.T) {
	t.Parallel()
	items, err := normalizedInput(t, `{"questions":[
		{"label":"Суперсила","type":"single_choice","question":"Какую силу?","description":"d","options":[]},
		{"id":"fly","label":"Полёт","description":"Летать","type":"single_choice"},
		{"id":"time","label":"Время","type":"single_choice"}
	]}`)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []QuestionChoice{{ID: "fly", Label: "Полёт", Description: "Летать"}, {ID: "time", Label: "Время"}}, items[0].Choices)
}

func TestQuestionInput_OnlyOptionsGivesShapeHint(t *testing.T) {
	t.Parallel()
	_, err := normalizedInput(t, `{"questions":[{"id":"gh","label":"GitHub Actions","type":"single_choice","description":"d"},{"id":"docker","label":"Docker","type":"single_choice"}]}`)
	require.ErrorContains(t, err, "looks like an answer option")
	require.ErrorContains(t, err, `"choices":[{"id":"tea"`)
}

func TestQuestionInput_MissingChoicesGivesShapeHint(t *testing.T) {
	t.Parallel()
	_, err := normalizedInput(t, `{"questions":[{"label":"Напиток","type":"single_choice","question":"Что пьём?","description":"d"}]}`)
	require.ErrorContains(t, err, "[Напиток]")
	require.ErrorContains(t, err, "nested inside it")
}

func TestQuestionInput_InfersTypeAndText(t *testing.T) {
	t.Parallel()
	items, err := normalizedInput(t, `{"questions":[{"label":"Цвет","description":"d","choices":[{"id":"r","label":"Red"},{"id":"g","label":"Green"}]},{"label":"Комментарий","description":"d"}]}`)
	require.NoError(t, err)
	require.Equal(t, "single_choice", items[0].Type)
	require.Equal(t, "Цвет", items[0].Question)
	require.Equal(t, "free_text", items[1].Type)
}

func TestQuestionParamsUnmarshalJSON_StringWithTrailingGarbage(t *testing.T) {
	t.Parallel()
	encoded, _ := json.Marshal(`[{"type":"yes_no","question":"OK?","description":"d"}]}`)
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(`{"questions": `+string(encoded)+`}`), &p))
	require.Len(t, p.Questions, 1)
}
