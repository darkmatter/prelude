package ui

import "strings"

// LetterSpace uppercases a section label for tracked group headers.
func LetterSpace(s string) string {
	return strings.ToUpper(s)
}
