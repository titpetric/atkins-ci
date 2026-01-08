package runner

import (
	"bytes"
	"io"
	"os"
	"os/exec"
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
type Exec struct {
	Env map[string]string // Optional environment variables to pass to commands
}

// NewExec creates a new Exec instance.
func NewExec() *Exec {
	return &Exec{
		Env: make(map[string]string),
	}
}

// NewExecWithEnv creates a new Exec instance with environment variables.
func NewExecWithEnv(env map[string]string) *Exec {
	return &Exec{
		Env: env,
	}
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

	// Build environment: start with OS environment, then overlay custom env
	cmdEnv := os.Environ()
	for k, v := range e.Env {
		// Remove existing key if present and add new one
		cmdEnv = removeEnvKey(cmdEnv, k)
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

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

		resErr := ExecError{
			Message:      "failed to run command: " + err.Error(),
			LastExitCode: exitCode,
			Output:       stderr.String(),
			Trace:        "", // Stack traces disabled by default
		}

		return "", resErr
	}

	return stdout.String(), nil
}

// ExecuteCommandWithWriter executes a command and writes stdout to the provided writer.
// Also returns the full stdout string for the caller.
func (e *Exec) ExecuteCommandWithWriter(cmdStr string, writer io.Writer) (string, error) {
	if cmdStr == "" {
		return "", nil
	}

	cmd := exec.Command("bash", "-c", cmdStr)

	// Build environment: start with OS environment, then overlay custom env
	cmdEnv := os.Environ()
	for k, v := range e.Env {
		// Remove existing key if present and add new one
		cmdEnv = removeEnvKey(cmdEnv, k)
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

	// Write to both the provided writer and a buffer for the return value
	var stdout bytes.Buffer
	multiWriter := io.MultiWriter(&stdout, writer)
	cmd.Stdout = multiWriter

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Extract exit code
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		resErr := ExecError{
			Message:      "failed to run command: " + err.Error(),
			LastExitCode: exitCode,
			Output:       stderr.String(),
			Trace:        "", // Stack traces disabled by default
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
