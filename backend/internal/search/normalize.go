package search

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func Normalize(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(value))
	var builder strings.Builder
	builder.Grow(len(decomposed))
	lastSpace := true
	for _, character := range decomposed {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsSpace(character) {
			if !lastSpace {
				builder.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		builder.WriteRune(character)
		lastSpace = false
	}
	return strings.TrimSpace(builder.String())
}

func SearchText(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if normalized := Normalize(value); normalized != "" {
			parts = append(parts, normalized)
		}
	}
	return strings.Join(parts, " ")
}
