# Package ./colors

```go
import (
	"github.com/titpetric/atkins/colors"
}
```

## Function symbols

- `func BrightCyan (text string) string`
- `func BrightGreen (text string) string`
- `func BrightMagenta (text string) string`
- `func BrightOrange (text string) string`
- `func BrightRed (text string) string`
- `func BrightWhite (text string) string`
- `func BrightYellow (text string) string`
- `func Dim (text string) string`
- `func Gray (text string) string`
- `func Green (text string) string`
- `func PrintFail (name,errMsg string)`
- `func PrintHeader (title string)`
- `func PrintInfo (key,value string)`
- `func PrintPass (name string)`
- `func PrintSectionEnd (section string, success bool)`
- `func PrintSectionStart (section string)`
- `func PrintSuccess (text string)`
- `func PrintWarning (text string)`
- `func StripANSI (in string) string`
- `func VisualLength (s string) int`
- `func White (text string) string`

### BrightCyan

BrightCyan returns text in bright cyan color.

```go
func BrightCyan (text string) string
```

### BrightGreen

BrightGreen returns text in bright green color.

```go
func BrightGreen (text string) string
```

### BrightMagenta

BrightMagenta returns text in bright magenta color.

```go
func BrightMagenta (text string) string
```

### BrightOrange

BrightOrange returns text in bright orange color.

```go
func BrightOrange (text string) string
```

### BrightRed

BrightRed returns text in bright red color.

```go
func BrightRed (text string) string
```

### BrightWhite

BrightWhite returns text in bright white color.

```go
func BrightWhite (text string) string
```

### BrightYellow

BrightYellow returns text in bright yellow color.

```go
func BrightYellow (text string) string
```

### Dim

Dim returns text in dim color.

```go
func Dim (text string) string
```

### Gray

Gray returns text in gray color.

```go
func Gray (text string) string
```

### Green

Green returns text in green color.

```go
func Green (text string) string
```

### PrintFail

PrintFail prints a failed item with error message.

```go
func PrintFail (name,errMsg string)
```

### PrintHeader

PrintHeader prints a header with bright cyan color.

```go
func PrintHeader (title string)
```

### PrintInfo

PrintInfo prints an information line.

```go
func PrintInfo (key,value string)
```

### PrintPass

PrintPass prints a passing item.

```go
func PrintPass (name string)
```

### PrintSectionEnd

PrintSectionEnd prints a section end marker.

```go
func PrintSectionEnd (section string, success bool)
```

### PrintSectionStart

PrintSectionStart prints a section start marker.

```go
func PrintSectionStart (section string)
```

### PrintSuccess

PrintSuccess prints a success message.

```go
func PrintSuccess (text string)
```

### PrintWarning

PrintWarning prints a warning message.

```go
func PrintWarning (text string)
```

### StripANSI

StripANSI removes all ANSI escape sequences from a string.

```go
func StripANSI (in string) string
```

### VisualLength

VisualLength returns the visual length of a string (excluding ANSI sequences).

```go
func VisualLength (s string) int
```

### White

White returns text in white color.

```go
func White (text string) string
```


