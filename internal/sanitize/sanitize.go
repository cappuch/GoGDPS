package sanitize

import (
	"html"
	"regexp"
	"strings"
)

var charCleanRe = regexp.MustCompile(`[^A-Za-z0-9 ]`)
var numberColonRe = regexp.MustCompile(`[^0-9,-]`)
var numberRe = regexp.MustCompile(`[^0-9]`)

// Remove mirrors ExploitPatch::remove — strips dangerous characters from user input.
func Remove(s string) string {
	s = strings.TrimSpace(s)
	s = html.EscapeString(s)
	for _, sep := range []string{":", "|", "~", "#"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			s = s[:idx]
		}
	}
	s = strings.ReplaceAll(s, "\x00", "")
	if idx := strings.Index(s, ")"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// CharClean keeps only alphanumeric characters and spaces.
func CharClean(s string) string {
	return charCleanRe.ReplaceAllString(s, "")
}

// NumberColon keeps digits, commas, and hyphens.
func NumberColon(s string) string {
	return numberColonRe.ReplaceAllString(s, "")
}

// Number keeps only digits.
func Number(s string) string {
	return numberRe.ReplaceAllString(s, "")
}
