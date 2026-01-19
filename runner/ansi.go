package runner

import (
	"regexp"
	"strconv"
	"strings"
)

// csiPattern matches ANSI CSI escape sequences (for length calculation).
var csiPattern = regexp.MustCompile(`\x1b\[[?>]?[0-9;]*[A-Za-z]`)

// cursorUpClearPattern matches cursor up + clear to end of display: \033[<n>A\033[J
// This is used by treeview/display to redraw the tree.
var cursorUpClearPattern = regexp.MustCompile(`\x1b\[(\d+)A\x1b\[J`)

// Sanitize processes raw terminal output and returns clean lines.
// It handles:
// - Cursor up + clear sequences (\033[nA\033[J) used by treeview
// - Carriage returns (\r) by taking content after the last \r
// - CRLF normalization
// - Preserves ANSI color sequences in output
//
// Returns sanitized lines with colors preserved.
func Sanitize(in string) ([]string, error) {
	if in == "" {
		return nil, nil
	}

	// Normalize \r\n to \n first (before any other processing)
	clean := strings.ReplaceAll(in, "\r\n", "\n")

	// Process cursor up + clear sequences
	// These indicate "go back n lines and clear everything from there"
	clean = processCursorUpClear(clean)

	// Process line by line, handling carriage returns
	rawLines := strings.Split(clean, "\n")
	result := make([]string, 0, len(rawLines))

	for _, rawLine := range rawLines {
		line := processCarriageReturns(rawLine)
		line = strings.TrimRight(line, " \t\r")
		result = append(result, line)
	}

	// Filter out empty lines
	filtered := make([]string, 0, len(result))
	for _, line := range result {
		if line != "" {
			filtered = append(filtered, line)
		}
	}

	return filtered, nil
}

// processCursorUpClear handles \033[nA\033[J sequences.
// These move cursor up n lines and clear from cursor to end of display.
// We simulate this by removing the last n lines before the sequence.
func processCursorUpClear(in string) string {
	for {
		loc := cursorUpClearPattern.FindStringSubmatchIndex(in)
		if loc == nil {
			break
		}

		// Extract n (number of lines to go up)
		nStr := in[loc[2]:loc[3]]
		n, _ := strconv.Atoi(nStr)

		// Find position of the escape sequence
		escStart := loc[0]
		escEnd := loc[1]

		// Count back n newlines from escStart to find where to cut
		// We need to find the start of line that is n lines back
		cutPos := escStart
		linesFound := 0

		// First, if we're right after a newline, that doesn't count as a line to go back
		// Move to find actual content lines
		for cutPos > 0 && linesFound < n {
			cutPos--
			if in[cutPos] == '\n' {
				linesFound++
			}
		}

		// Now cutPos is either at a \n or at position 0
		// We want to cut from the start of the line we landed on
		if cutPos > 0 {
			// We're at a \n, so the cut starts after this \n
			// But we need to go back one more to get the start of that line
			// Actually, we want to keep everything before this line
			// Find the previous newline to keep
			searchPos := cutPos - 1
			for searchPos >= 0 && in[searchPos] != '\n' {
				searchPos--
			}
			cutPos = searchPos + 1 // Start of the line to remove
		} else {
			cutPos = 0
		}

		// Remove from cutPos to escEnd (the cleared content + escape sequence)
		in = in[:cutPos] + in[escEnd:]
	}

	return in
}

// processCarriageReturns handles \r characters in a line.
// When \r is encountered, the cursor returns to the beginning of the line
// and subsequent characters overwrite from position 0.
// This function properly handles ANSI escape sequences (they don't take visual space).
func processCarriageReturns(line string) string {
	if !strings.Contains(line, "\r") {
		return line
	}

	// Split by \r
	segments := strings.Split(line, "\r")

	// Build result by overlaying segments
	// Each segment after a \r starts writing from position 0
	var result []rune
	var resultLen int // Visual length (excluding ANSI sequences)

	for _, seg := range segments {
		// Parse segment into visual characters and escape sequences
		pos := 0 // Current visual position for writing
		i := 0
		segRunes := []rune(seg)

		for i < len(segRunes) {
			// Check if we're at the start of an ANSI escape sequence
			if segRunes[i] == '\x1b' && i+1 < len(segRunes) && segRunes[i+1] == '[' {
				// Find the end of the CSI sequence
				escStart := i
				i += 2 // Skip ESC [
				for i < len(segRunes) && !isCSIFinalByte(byte(segRunes[i])) {
					i++
				}
				if i < len(segRunes) {
					i++ // Include final byte
				}
				// Insert escape sequence at current position without advancing visual position
				escSeq := segRunes[escStart:i]
				result = insertEscapeSequence(result, pos, escSeq)
			} else {
				// Regular character - write at current visual position
				if pos < resultLen {
					// Overwrite existing character
					result = setRuneAtVisualPos(result, pos, segRunes[i])
				} else {
					// Append new character
					result = append(result, segRunes[i])
					resultLen++
				}
				pos++
				i++
			}
		}
	}

	return string(result)
}

// isCSIFinalByte returns true if b is a valid CSI final byte (0x40-0x7E).
func isCSIFinalByte(b byte) bool {
	return b >= 0x40 && b <= 0x7E
}

// insertEscapeSequence inserts an escape sequence at the given visual position.
// Escape sequences don't occupy visual space.
func insertEscapeSequence(result []rune, visualPos int, escSeq []rune) []rune {
	// Find the actual rune index for this visual position
	idx := visualPosToRuneIndex(result, visualPos)
	if idx >= len(result) {
		return append(result, escSeq...)
	}
	// Insert escape sequence at this position
	newResult := make([]rune, 0, len(result)+len(escSeq))
	newResult = append(newResult, result[:idx]...)
	newResult = append(newResult, escSeq...)
	newResult = append(newResult, result[idx:]...)
	return newResult
}

// setRuneAtVisualPos sets a rune at the given visual position.
func setRuneAtVisualPos(result []rune, visualPos int, r rune) []rune {
	idx := visualPosToRuneIndex(result, visualPos)
	if idx < len(result) {
		// Check if we're overwriting an escape sequence - skip it
		for idx < len(result) && result[idx] == '\x1b' {
			// Skip the entire escape sequence
			idx++
			if idx < len(result) && result[idx] == '[' {
				idx++
				for idx < len(result) && !isCSIFinalByte(byte(result[idx])) {
					idx++
				}
				if idx < len(result) {
					idx++
				}
			}
		}
		if idx < len(result) {
			result[idx] = r
		} else {
			result = append(result, r)
		}
	}
	return result
}

// visualPosToRuneIndex converts a visual position to a rune index,
// skipping over ANSI escape sequences.
func visualPosToRuneIndex(result []rune, visualPos int) int {
	idx := 0
	pos := 0
	for idx < len(result) && pos < visualPos {
		if result[idx] == '\x1b' && idx+1 < len(result) && result[idx+1] == '[' {
			// Skip escape sequence
			idx += 2
			for idx < len(result) && !isCSIFinalByte(byte(result[idx])) {
				idx++
			}
			if idx < len(result) {
				idx++
			}
		} else {
			pos++
			idx++
		}
	}
	// Skip any trailing escape sequences at this position
	for idx < len(result) && result[idx] == '\x1b' && idx+1 < len(result) && result[idx+1] == '[' {
		idx += 2
		for idx < len(result) && !isCSIFinalByte(byte(result[idx])) {
			idx++
		}
		if idx < len(result) {
			idx++
		}
	}
	return idx
}

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(in string) string {
	return csiPattern.ReplaceAllString(in, "")
}

// VisualLength returns the visual length of a string (excluding ANSI sequences).
func VisualLength(s string) int {
	stripped := StripANSI(s)
	return len([]rune(stripped))
}
