package colors

import "fmt"

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorBright  = "\033[1m"
	colorDim     = "\033[2m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorRed     = "\033[31m"
)

func colorize(color, text string) string {
	return color + text + colorReset
}

func BrightGreen(text string) string {
	return colorBright + colorGreen + text + colorReset
}

func BrightYellow(text string) string {
	return colorBright + colorYellow + text + colorReset
}

func BrightCyan(text string) string {
	return colorBright + colorCyan + text + colorReset
}

func BrightMagenta(text string) string {
	return colorBright + colorMagenta + text + colorReset
}

func BrightRed(text string) string {
	return colorBright + colorRed + text + colorReset
}

func Dim(text string) string {
	return colorDim + text + colorReset
}

func PrintHeader(title string) {
	fmt.Printf("# %s\n", BrightCyan(title))
}

func PrintSectionStart(section string) {
	fmt.Printf("%s %s\n", colorize(colorMagenta, "▶"), BrightMagenta(section))
}

func PrintSectionEnd(section string, success bool) {
	if success {
		fmt.Printf("%s %s\n", colorize(colorGreen, "✓"), BrightGreen(section))
	} else {
		fmt.Printf("%s %s\n", colorize(colorRed, "✗"), BrightRed(section))
	}
}

func PrintPass(name string) {
	fmt.Printf(" %s %s\n", colorize(colorGreen, "[✓]"), BrightGreen(name))
}

func PrintFail(name, errMsg string) {
	fmt.Printf(" %s %s\n", colorize(colorRed, "[✗]"), BrightRed(name))
	fmt.Printf("    %s %s\n", colorize(colorRed, "→"), colorize(colorRed, errMsg))
}

func PrintInfo(key, value string) {
	fmt.Printf(" %s %s %s\n", colorize(colorCyan, "●"), BrightCyan(key+":"), colorize(colorYellow, value))
}

func PrintSuccess(text string) {
	fmt.Printf("%s %s\n", colorize(colorGreen, ">>"), BrightGreen(text))
}

func PrintWarning(text string) {
	fmt.Printf("%s %s\n", colorize(colorYellow, "⚠"), BrightYellow(text))
}
