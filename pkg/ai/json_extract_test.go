package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"simple object",
			`{"key": "value"}`,
			`{"key": "value"}`,
		},
		{
			"object with prefix",
			`prefix {"key": "value"}`,
			`{"key": "value"}`,
		},
		{
			"object with suffix",
			`{"key": "value"} suffix`,
			`{"key": "value"}`,
		},
		{
			"nested object",
			`{"outer": {"inner": "value"}}`,
			`{"outer": {"inner": "value"}}`,
		},
		{
			"nested with text",
			`text {"outer": {"inner": "value"}} more`,
			`{"outer": {"inner": "value"}}`,
		},
		{
			"array",
			`[1, 2, 3]`,
			`[1, 2, 3]`,
		},
		{
			"array with text",
			`items: [1, 2, 3] end`,
			`[1, 2, 3]`,
		},
		{
			"nested array in object",
			`{"items": [1, {"nested": 2}, 3]}`,
			`{"items": [1, {"nested": 2}, 3]}`,
		},
		{
			"string with braces",
			`{"text": "hello {world}"}`,
			`{"text": "hello {world}"}`,
		},
		{
			"escaped quotes",
			`{"text": "he said \"hello\""}`,
			`{"text": "he said \"hello\""}`,
		},
		{
			"empty",
			``,
			``,
		},
		{
			"no json",
			`just text`,
			``,
		},
		{
			"multiple objects - first one",
			`{"first": 1} {"second": 2}`,
			`{"first": 1}`,
		},
		{
			"object with newlines",
			`{
  "key": "value"
}`,
			`{
  "key": "value"
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractJSON(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
