package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ExecuteCommand executes a shell command using bash -c
func ExecuteCommand(cmdStr string) (string, error) {
	if cmdStr == "" {
		return "", nil
	}

	cmd := exec.Command("bash", "-c", cmdStr)

	// Inherit current process environment
	cmd.Env = os.Environ()

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// ExecuteCommandWithEnv executes a shell command with custom environment
func ExecuteCommandWithEnv(cmdStr string, env map[string]string) (string, error) {
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
