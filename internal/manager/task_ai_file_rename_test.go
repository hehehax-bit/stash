package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World.mp4", "Hello World.mp4"},
		{"path/to/file:bad*.txt", "path to file bad .txt"},
		{"trailing...dot.", "trailing...dot"},
		{"  spaced  out  ", "spaced out"},
		{"", ""},
		{"file<>name", "file name"},
		{"file?name", "file name"},
		{"file\"name", "file name"},
		{"file|name", "file name"},
		{"file\\name", "file name"},
		{"file/name", "file name"},
		{"file:name", "file name"},
		{"file*name", "file name"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSanitizeFilenameTruncation(t *testing.T) {
	long := "a" + "b"
	got := sanitizeFilename(long)
	assert.LessOrEqual(t, len(got), 80)
	assert.NotRegexp(t, `[\\/\*\?":<>|\n\r\t]`, got)
	assert.NotRegexp(t, `[. ]+$`, got)
}

func TestSanitizeFilenameControlChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"file\ttab", "file tab"},
		{"file\nnewline", "file newline"},
		{"file\rreturn", "file return"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSanitizeFilenameNoTrailingDots(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"file...", "file"},
		{"file..", "file"},
		{"file.", "file"},
		{"...", ""},
		{".hidden", "hidden"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
