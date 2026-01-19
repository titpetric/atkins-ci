package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitize_EmptyInput(t *testing.T) {
	lines, err := Sanitize("")
	require.NoError(t, err)
	assert.Nil(t, lines)
}

func TestSanitize_PlainText(t *testing.T) {
	lines, err := Sanitize("hello\nworld")
	require.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, lines)
}

func TestSanitize_ColorSequencesPreserved(t *testing.T) {
	// Colors should be preserved in output
	input := "\033[33m∅\033[0m  colors\n\033[32m✓\033[0m  model (6ms)"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"\033[33m∅\033[0m  colors", "\033[32m✓\033[0m  model (6ms)"}, lines)
}

func TestSanitize_CarriageReturnClearsLine(t *testing.T) {
	// \r clears content before it, keeping what comes after
	input := "processing...\rDONE 10 tests"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"DONE 10 tests"}, lines)
}

func TestSanitize_CarriageReturnPartialOverwrite(t *testing.T) {
	// \r followed by shorter text should overwrite beginning
	input := "hello world\rfoo"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"foolo world"}, lines)
}

func TestSanitize_CarriageReturnWithColors(t *testing.T) {
	// Colors should be preserved after \r processing
	input := "old text\r\033[32mnew\033[0m"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"\033[32mnew\033[0m text"}, lines)
}

func TestSanitize_256ColorPreserved(t *testing.T) {
	// 256-color sequence should be preserved
	input := "\033[38;5;208morange\033[0m text"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"\033[38;5;208morange\033[0m text"}, lines)
}

func TestSanitize_TrailingNewlines(t *testing.T) {
	input := "hello\n\n\n"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"hello"}, lines)
}

func TestSanitize_LeadingNewlines(t *testing.T) {
	input := "\n\nhello"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"hello"}, lines)
}

func TestSanitize_CursorUpClear(t *testing.T) {
	// Treeview uses \033[nA\033[J to move cursor up n lines and clear to end of display
	// This simulates redrawing: first render "line1\nline2\n", then clear 2 lines and write "new1\nnew2"
	input := "line1\nline2\n\033[2A\033[Jnew1\nnew2"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"new1", "new2"}, lines)
}

func TestSanitize_CursorUpClearPartial(t *testing.T) {
	// Clear only 1 line, keeping earlier content
	input := "keep1\nkeep2\nremove\n\033[1A\033[Jreplaced"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"keep1", "keep2", "replaced"}, lines)
}

func TestSanitize_CursorUpClearMultiple(t *testing.T) {
	// Multiple cursor up + clear sequences (progressive updates)
	input := "v1\n\033[1A\033[Jv2\n\033[1A\033[Jv3"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"v3"}, lines)
}

func TestSanitize_CursorUpClearWithColors(t *testing.T) {
	// Colors should be preserved through cursor up + clear
	input := "\033[32mold\033[0m\n\033[1A\033[J\033[33mnew\033[0m"
	lines, err := Sanitize(input)
	require.NoError(t, err)
	assert.Equal(t, []string{"\033[33mnew\033[0m"}, lines)
}

func TestSanitize_GotestsumWithCRLF(t *testing.T) {
	// Real gotestsum output with CRLF line endings
	input := "\033[33m∅\033[0m  colors\r\n" +
		"\033[32m✓\033[0m  model (12ms)\r\n" +
		"\r\n" +
		"DONE 10 tests\r\n"

	lines, err := Sanitize(input)
	require.NoError(t, err)
	expected := []string{
		"\033[33m∅\033[0m  colors",
		"\033[32m✓\033[0m  model (12ms)",
		"DONE 10 tests",
	}
	assert.Equal(t, expected, lines)
}

func TestStripANSI(t *testing.T) {
	input := "\033[1m\033[32mgreen\033[0m"
	result := StripANSI(input)
	assert.Equal(t, "green", result)
}

func TestVisualLength(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"\033[32mhello\033[0m", 5},
		{"∅  colors", 9},
		{"\033[33m∅\033[0m  colors", 9},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, VisualLength(tt.input))
		})
	}
}
