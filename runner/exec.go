package runner

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ExecError represents an error from command execution.
type ExecError struct {
	Message      string
	Output       string
	LastExitCode int
	Trace        string
}

// Error returns the error message.
func (r ExecError) Error() string {
	return r.Message
}

// Len returns the length of the error message.
func (r ExecError) Len() int {
	return len(r.Message)
}

// Exec runs shell commands.
type Exec struct{}

// NewExec creates a new Exec instance.
func NewExec() *Exec {
	return &Exec{}
}

// ExecuteCommand will run the command quietly.
func (e *Exec) ExecuteCommand(cmdStr string) (string, error) {
	return e.ExecuteCommandWithQuiet(cmdStr, false)
}

// ExecuteCommandWithQuiet executes a shell command with quiet mode.
func (e *Exec) ExecuteCommandWithQuiet(cmdStr string, verbose bool) (string, error) {
	return e.ExecuteCommandWithQuietAndCapture(cmdStr, verbose)
}

// ExecuteCommandWithQuietAndCapture executes a shell command with quiet mode and captures stderr.
// Returns (stdout, error). If error occurs, stderr is logged to the global buffer.
func (e *Exec) ExecuteCommandWithQuietAndCapture(cmdStr string, verbose bool) (string, error) {
	if cmdStr == "" {
		return "", nil
	}

	cmd := exec.Command("bash", "-c", cmdStr)

	// Inherit current process environment
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Extract exit code
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		b := make([]byte, 2048) // adjust buffer size to be larger than expected stack
		n := runtime.Stack(b, false)
		s := string(b[:n])

		resErr := ExecError{
			Message:      "failed to run command: " + err.Error(),
			LastExitCode: exitCode,
			Output:       stderr.String(),
			Trace:        s,
		}

		return "", resErr
	}

	return stdout.String(), nil
}

// removeEnvKey removes a key from environment variable list
func removeEnvKey(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}
