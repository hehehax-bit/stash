package ai

import (
	"strings"
)

// ExtractJSON extracts the first valid JSON object or array from a string.
// It handles nested braces/brackets correctly by tracking depth.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}

	start := -1
	var openChar byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' || s[i] == '[' {
			start = i
			openChar = s[i]
			break
		}
	}
	if start == -1 {
		return ""
	}

	closeChar := byte('}')
	if openChar == '[' {
		closeChar = ']'
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case openChar:
			depth++
		case closeChar:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
