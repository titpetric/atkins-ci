package colors

import (
	"fmt"
	"regexp"
)

// csiPattern matches ANSI CSI escape sequences.
var csiPattern = regexp.MustCompile(`\x1b\[[?>]?[0-9;]*[A-Za-z]`)

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(in string) string {
	return csiPattern.ReplaceAllString(in, "")
}

// VisualLength returns the visual length of a string (excluding ANSI sequences).
func VisualLength(s string) int {
	stripped := StripANSI(s)
	return len([]rune(stripped))
}

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorBright  = "\033[1m"
	colorDim     = "\033[2m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorOrange  = "\033[38;5;208m"
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorRed     = "\033[31m"
	colorWhite   = "\033[37m"
	colorGray    = "\033[90m"
)

func colorize(color, text string) string {
	return color + text + colorReset
}

// BrightGreen returns text in bright green color.
func BrightGreen(text string) string {
	return colorBright + colorGreen + text + colorReset
}

// Green returns text in green color.
func Green(text string) string {
	return colorGreen + text + colorReset
}

// BrightYellow returns text in bright yellow color.
func BrightYellow(text string) string {
	return colorBright + colorYellow + text + colorReset
}

// BrightOrange returns text in bright orange color.
func BrightOrange(text string) string {
	return colorOrange + text + colorReset
}

// BrightCyan returns text in bright cyan color.
func BrightCyan(text string) string {
	return colorBright + colorCyan + text + colorReset
}

// BrightMagenta returns text in bright magenta color.
func BrightMagenta(text string) string {
	return colorBright + colorMagenta + text + colorReset
}

// BrightRed returns text in bright red color.
func BrightRed(text string) string {
	return colorBright + colorRed + text + colorReset
}

// Dim returns text in dim color.
func Dim(text string) string {
	return colorDim + text + colorReset
}

// BrightWhite returns text in bright white color.
func BrightWhite(text string) string {
	return colorBright + colorWhite + text + colorReset
}

// White returns text in white color.
func White(text string) string {
	return colorWhite + text + colorReset
}

// Gray returns text in gray color.
func Gray(text string) string {
	return colorGray + text + colorReset
}

// PrintHeader prints a header with bright cyan color.
func PrintHeader(title string) {
	fmt.Printf("# %s\n", BrightCyan(title))
}

// PrintSectionStart prints a section start marker.
func PrintSectionStart(section string) {
	fmt.Printf("%s %s\n", colorize(colorMagenta, "▶"), BrightMagenta(section))
}

// PrintSectionEnd prints a section end marker.
func PrintSectionEnd(section string, success bool) {
	if success {
		fmt.Printf("%s %s\n", colorize(colorGreen, "✓"), BrightGreen(section))
	} else {
		fmt.Printf("%s %s\n", colorize(colorRed, "✗"), BrightRed(section))
	}
}

// PrintPass prints a passing item.
func PrintPass(name string) {
	fmt.Printf(" %s %s\n", colorize(colorGreen, "[✓]"), BrightGreen(name))
}

// PrintFail prints a failed item with error message.
func PrintFail(name, errMsg string) {
	fmt.Printf(" %s %s\n", colorize(colorRed, "[✗]"), BrightRed(name))
	fmt.Printf("    %s %s\n", colorize(colorRed, "→"), colorize(colorRed, errMsg))
}

// PrintInfo prints an information line.
func PrintInfo(key, value string) {
	fmt.Printf(" %s %s %s\n", colorize(colorCyan, "●"), BrightCyan(key+":"), colorize(colorYellow, value))
}

// PrintSuccess prints a success message.
func PrintSuccess(text string) {
	fmt.Printf("%s %s\n", colorize(colorGreen, ">>"), BrightGreen(text))
}

// PrintWarning prints a warning message.
func PrintWarning(text string) {
	fmt.Printf("%s %s\n", colorize(colorYellow, "⚠"), BrightYellow(text))
}
