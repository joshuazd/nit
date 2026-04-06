package diff

import (
	"regexp"
	"strings"

	diffmatch "github.com/sergi/go-diff/diffmatchpatch"
)

var wordSplitRE = regexp.MustCompile(`\S+|\s+`)

// DiffSegment represents a piece of text and whether it changed.
type DiffSegment struct {
	Text    string
	Changed bool
}

func tokenize(text string) []string {
	return wordSplitRE.FindAllString(text, -1)
}

// WordDiffSegments computes word-level diff between two strings.
// Returns two lists of segments (one for old, one for new).
func WordDiffSegments(oldText, newText string) ([]DiffSegment, []DiffSegment) {
	oldTokens := tokenize(oldText)
	newTokens := tokenize(newText)

	// Use diffmatchpatch on the joined token representation.
	// We join with a zero-width separator that won't appear in real text,
	// then diff at the character level on the token-boundary text.
	// Actually, simpler: convert tokens to single-char representation for diffing.
	dmp := diffmatch.New()

	// Map tokens to unique runes for efficient diff
	tokenMap := make(map[string]rune)
	var nextRune rune = 0xE000 // Start in Unicode private use area

	mapTokens := func(tokens []string) string {
		var b strings.Builder
		for _, t := range tokens {
			if r, ok := tokenMap[t]; ok {
				b.WriteRune(r)
			} else {
				tokenMap[t] = nextRune
				b.WriteRune(nextRune)
				nextRune++
			}
		}
		return b.String()
	}

	oldMapped := mapTokens(oldTokens)
	newMapped := mapTokens(newTokens)

	// Reverse map: rune -> token string
	reverseMap := make(map[rune]string)
	for s, r := range tokenMap {
		reverseMap[r] = s
	}

	diffs := dmp.DiffMain(oldMapped, newMapped, false)

	var oldSegs, newSegs []DiffSegment
	for _, d := range diffs {
		text := expandRunes(d.Text, reverseMap)
		switch d.Type {
		case diffmatch.DiffEqual:
			oldSegs = append(oldSegs, DiffSegment{Text: text, Changed: false})
			newSegs = append(newSegs, DiffSegment{Text: text, Changed: false})
		case diffmatch.DiffDelete:
			oldSegs = append(oldSegs, DiffSegment{Text: text, Changed: true})
		case diffmatch.DiffInsert:
			newSegs = append(newSegs, DiffSegment{Text: text, Changed: true})
		}
	}

	return oldSegs, newSegs
}

func expandRunes(s string, m map[rune]string) string {
	var b strings.Builder
	for _, r := range s {
		if t, ok := m[r]; ok {
			b.WriteString(t)
		}
	}
	return b.String()
}
