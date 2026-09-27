package voice

import (
	"strings"
	"unicode"
)

// languageCandidates splits a configured language setting into the ordered
// list of tags to try. "auto" and blanks drop out, tags are lowercased and
// duplicates removed, and the order survives because it is the order engines
// which must choose a language themselves search in.
//
// A setting of "ru-RU" is one fixed language. A setting of "ru-RU,en-US" asks
// for a choice between the two. An empty result means "no preference", which
// leaves detection to the engine.
func languageCandidates(language string) []string {
	parts := strings.Split(language, ",")
	candidates := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag == "" || tag == "auto" || seen[tag] {
			continue
		}
		seen[tag] = true
		candidates = append(candidates, tag)
	}
	return candidates
}

// soleLanguage returns the configured tag when exactly one language is set, and
// an empty string otherwise. Engines that accept a single language code (the
// OpenAI-compatible ones) need this: a list is not a code, and for them "no
// code" already means "detect it yourself".
func soleLanguage(language string) string {
	candidates := languageCandidates(language)
	if len(candidates) != 1 {
		return ""
	}
	return candidates[0]
}

// script is a writing system. It is what separates a transcript in the language
// that was asked for from another language's fluent nonsense: the same Russian
// audio comes back as Cyrillic from the Russian model and as Latin gibberish
// from the English one.
type script int

const (
	scriptUnknown script = iota
	scriptLatin
	scriptCyrillic
	scriptGreek
	scriptArabic
	scriptHebrew
	scriptDevanagari
	scriptThai
	scriptHan
	scriptKana
	scriptHangul
)

// languageScripts maps the primary subtag of a BCP-47 tag to the writing system
// that language is normally written in. Languages Prime has no opinion about
// are absent on purpose, and an unknown language never loses a comparison.
var languageScripts = map[string]script{
	"ar": scriptArabic, "fa": scriptArabic, "ur": scriptArabic,
	"be": scriptCyrillic, "bg": scriptCyrillic, "kk": scriptCyrillic,
	"ky": scriptCyrillic, "mk": scriptCyrillic, "mn": scriptCyrillic,
	"ru": scriptCyrillic, "sr": scriptCyrillic, "tg": scriptCyrillic,
	"uk": scriptCyrillic,
	"el": scriptGreek,
	"he": scriptHebrew, "iw": scriptHebrew,
	"hi": scriptDevanagari, "mr": scriptDevanagari, "ne": scriptDevanagari,
	"th": scriptThai,
	"zh": scriptHan,
	"ja": scriptKana,
	"ko": scriptHangul,
	"af": scriptLatin, "ca": scriptLatin, "cs": scriptLatin, "cy": scriptLatin,
	"da": scriptLatin, "de": scriptLatin, "en": scriptLatin, "es": scriptLatin,
	"et": scriptLatin, "eu": scriptLatin, "fi": scriptLatin, "fr": scriptLatin,
	"ga": scriptLatin, "gl": scriptLatin, "hr": scriptLatin, "hu": scriptLatin,
	"id": scriptLatin, "is": scriptLatin, "it": scriptLatin, "lt": scriptLatin,
	"lv": scriptLatin, "ms": scriptLatin, "mt": scriptLatin, "nl": scriptLatin,
	"no": scriptLatin, "pl": scriptLatin, "pt": scriptLatin, "ro": scriptLatin,
	"sk": scriptLatin, "sl": scriptLatin, "sq": scriptLatin, "sv": scriptLatin,
	"sw": scriptLatin, "tl": scriptLatin, "tr": scriptLatin, "vi": scriptLatin,
}

// expectedScript returns the writing system a language tag asks for. An empty
// result means Prime does not know the language, which leaves the comparison
// undecided rather than deciding it wrongly.
func expectedScript(language string) script {
	primary := strings.ToLower(strings.TrimSpace(language))
	if cut := strings.IndexAny(primary, "-_."); cut >= 0 {
		primary = primary[:cut]
	}
	return languageScripts[primary]
}

// onExpectedScript reports whether a transcript is written in the system its
// language is written in. A language Prime does not know is never counted as a
// mismatch, since there is nothing to mismatch.
func onExpectedScript(language, text string) bool {
	want := expectedScript(language)
	if want == scriptUnknown {
		return true
	}
	found := scriptOf(text)
	if found == scriptUnknown {
		return false
	}
	// Japanese writes kana next to Han characters, so either one counts.
	if want == scriptKana && (found == scriptKana || found == scriptHan) {
		return true
	}
	return found == want
}

// scriptOf returns the writing system that dominates the text. Characters that
// are not letters, such as digits and punctuation, are ignored.
func scriptOf(text string) script {
	counts := map[script]int{}
	for _, r := range text {
		if s := runeScript(r); s != scriptUnknown {
			counts[s]++
		}
	}
	best, bestCount := scriptUnknown, 0
	// Ranging over the constants keeps the tie-break stable: the first script
	// in declaration order wins an equal count.
	for s := scriptUnknown + 1; s <= scriptHangul; s++ {
		if counts[s] > bestCount {
			best, bestCount = s, counts[s]
		}
	}
	return best
}

// runeScript names the writing system a single letter belongs to.
func runeScript(r rune) script {
	if !unicode.IsLetter(r) {
		return scriptUnknown
	}
	switch {
	case unicode.Is(unicode.Latin, r):
		return scriptLatin
	case unicode.Is(unicode.Cyrillic, r):
		return scriptCyrillic
	case unicode.Is(unicode.Greek, r):
		return scriptGreek
	case unicode.Is(unicode.Arabic, r):
		return scriptArabic
	case unicode.Is(unicode.Hebrew, r):
		return scriptHebrew
	case unicode.Is(unicode.Devanagari, r):
		return scriptDevanagari
	case unicode.Is(unicode.Thai, r):
		return scriptThai
	case unicode.Is(unicode.Han, r):
		return scriptHan
	case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
		return scriptKana
	case unicode.Is(unicode.Hangul, r):
		return scriptHangul
	}
	return scriptUnknown
}
