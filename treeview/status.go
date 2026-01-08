package treeview

import "github.com/titpetric/atkins/colors"

// Status represents the execution status of a node.
type Status int

// Status constants.
const (
	StatusPending Status = iota
	StatusRunning
	StatusPassed
	StatusFailed
	StatusSkipped
	StatusConditional
)

// String returns a string representation of the Status.
func (s Status) String() string {
	switch s {
	case StatusRunning:
		return colors.BrightOrange("●")
	case StatusPassed:
		return colors.BrightGreen("✓")
	case StatusFailed:
		return colors.BrightRed("✗")
	case StatusSkipped:
		return colors.BrightYellow("⊘")
	case StatusConditional:
		return colors.Gray("●")
	default:
	}
	return ""
}
