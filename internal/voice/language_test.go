package voice

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestLanguageCandidates(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		language string
		want     []string
	}{
		"unset":        {language: "", want: []string{}},
		"auto":         {language: "auto", want: []string{}},
		"single":       {language: "ru-RU", want: []string{"ru-ru"}},
		"padded":       {language: "  ru-RU  ", want: []string{"ru-ru"}},
		"list":         {language: "ru-RU, en-US", want: []string{"ru-ru", "en-us"}},
		"auto in list": {language: "ru-RU,auto,,en-US", want: []string{"ru-ru", "en-us"}},
		"repeated tag": {language: "ru-RU,ru-RU", want: []string{"ru-ru"}},
		// "ru" and "ru-RU" are different tags, so both stay: a plain tag and a
		// regional one are answers from different models.
		"region and plain": {language: "ru-RU,ru", want: []string{"ru-ru", "ru"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.want, languageCandidates(testCase.language))
		})
	}
}

// The list keeps its order, because that order is the search order for engines
// which have to pick a language themselves.
func TestNormalizedLanguageKeepsListOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, "en-us,ru-ru", normalizedLanguage("en-US, ru-RU"))
	require.Equal(t, "ru", normalizedLanguage("RU"))
	require.Empty(t, normalizedLanguage("auto"))
}

// Engines which take a single language code get one, and a list reads as "no
// preference", which for them already means "detect it".
func TestSoleLanguage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "ru-ru", soleLanguage("ru-RU"))
	require.Empty(t, soleLanguage("ru-RU,en-US"))
	require.Empty(t, soleLanguage(""))
	require.Empty(t, soleLanguage("auto"))
}

func TestScriptOf(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		text string
		want script
	}{
		"cyrillic":   {text: "привет мир", want: scriptCyrillic},
		"latin":      {text: "hello world", want: scriptLatin},
		"accented":   {text: "Voilà l'été", want: scriptLatin},
		"greek":      {text: "γειά σου", want: scriptGreek},
		"japanese":   {text: "こんにちは", want: scriptKana},
		"digits":     {text: "2024 42", want: scriptUnknown},
		"mixed en":   {text: "hello, world! 42", want: scriptLatin},
		"cyr and en": {text: "привет hello", want: scriptCyrillic},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.want, scriptOf(testCase.text))
		})
	}
}

func TestOnExpectedScript(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		language string
		text     string
		want     bool
	}{
		"russian text, russian language": {language: "ru-RU", text: "привет", want: true},
		"russian text, english language": {language: "en-US", text: "привет", want: false},
		"english text, english language": {language: "en-US", text: "hello there", want: true},
		"english text, russian language": {language: "ru-RU", text: "hello there", want: false},
		"region ignored":                 {language: "ru", text: "привет", want: true},
		"underscore ignored":             {language: "ru_RU", text: "привет", want: true},
		"ukrainian cyrillic":             {language: "uk-UA", text: "привіт", want: true},
		"japanese kana":                  {language: "ja-JP", text: "こんにちは", want: true},
		"japanese with kanji":            {language: "ja-JP", text: "日本語", want: true},
		// A language Prime has no script for is never counted as a mismatch,
		// since there is nothing to mismatch.
		"unknown language": {language: "xx-XX", text: "whatever", want: true},
		"empty text":       {language: "ru-RU", text: "", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.want, onExpectedScript(testCase.language, testCase.text))
		})
	}
}

// A wrong model on this endpoint is often the more confident of the two, so the
// alphabet leads and confidence only breaks the tie.
func TestBetterPrefersAlphabetOverConfidence(t *testing.T) {
	t.Parallel()

	own, lowConfidence := attempt{text: "привет", confidence: 0.4, onScript: true}, attempt{text: "hello", confidence: 0.9, onScript: false}

	require.True(t, better(own, lowConfidence))
	require.False(t, better(lowConfidence, own))
	require.True(t, better(attempt{}, attempt{}), "anything beats no answer")
	require.False(t, better(attempt{text: "a", confidence: 0.5}, attempt{text: "b", confidence: 0.5}))
}
