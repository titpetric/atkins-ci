package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ErrorLog captures stderr output from failed commands
var ErrorLog bytes.Buffer
var ErrorLogMutex sync.Mutex

// LastExitCode stores the last exit code from a failed command
var LastExitCode int

// ExecuteCommand executes a shell command using bash -c
func ExecuteCommand(cmdStr string) (string, error) {
	return ExecuteCommandWithQuiet(cmdStr, 0)
}

// ExecuteCommandWithQuiet executes a shell command with quiet mode
// quietMode: 0 = normal, 1 = suppress stdout, 2 = suppress stdout and stderr
func ExecuteCommandWithQuiet(cmdStr string, quietMode int) (string, error) {
	return ExecuteCommandWithQuietAndCapture(cmdStr, quietMode)
}

// ExecuteCommandWithQuietAndCapture executes a shell command with quiet mode and captures stderr
// Returns (stdout, error). If error occurs, stderr is logged to the global buffer.
func ExecuteCommandWithQuietAndCapture(cmdStr string, quietMode int) (string, error) {
	if cmdStr == "" {
		return "", nil
	}

	cmd := exec.Command("bash", "-c", cmdStr)

	// Inherit current process environment
	cmd.Env = os.Environ()

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	if err != nil {
		// Extract exit code
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		LastExitCode = exitCode

		// Log stderr to global error buffer if there's an error
		if stderrStr != "" {
			ErrorLogMutex.Lock()
			ErrorLog.WriteString(stderrStr)
			ErrorLogMutex.Unlock()
		}
		return stdoutStr, fmt.Errorf("command failed: %w", err)
	}

	return "", nil
}

// ExecuteCommandWithEnv executes a shell command with custom environment
func ExecuteCommandWithEnv(cmdStr string, env map[string]string) (string, error) {
	return ExecuteCommandWithEnvAndQuiet(cmdStr, env, 0)
}

// ExecuteCommandWithEnvAndQuiet executes a shell command with custom environment and quiet mode
// quietMode: 0 = normal, 1 = suppress stdout, 2 = suppress stdout and stderr
func ExecuteCommandWithEnvAndQuiet(cmdStr string, env map[string]string, quietMode int) (string, error) {
	if cmdStr == "" {
		return "", nil
	}

	cmd := exec.Command("bash", "-c", cmdStr)

	// Build environment slice
	envList := os.Environ()
	for k, v := range env {
		// Remove existing key if present
		envList = removeEnvKey(envList, k)
		// Add new value
		envList = append(envList, k+"="+v)
	}
	cmd.Env = envList

	if quietMode == 2 {
		// Very quiet: suppress all output to stderr too
		cmd.Stdout = nil
		cmd.Stderr = nil
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("command failed: %w", err)
		}
		return "", nil
	}

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
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
