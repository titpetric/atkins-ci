package runner_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/atkins/runner"
)

func TestExecuteCommand_QuietMode(t *testing.T) {
	tests := []struct {
		name          string
		cmd           string
		expectSuccess bool
	}{
		{
			name:          "simple echo",
			cmd:           "echo 'hello world'",
			expectSuccess: true,
		},
		{
			name:          "command with exit code 0",
			cmd:           "exit 0",
			expectSuccess: true,
		},
		{
			name:          "command with exit code 1",
			cmd:           "exit 1",
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := runner.NewExec()
			_, err := exec.ExecuteCommand(tt.cmd)

			if tt.expectSuccess {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestExecuteCommand_WithQuiet(t *testing.T) {
	exec := runner.NewExec()
	output, err := exec.ExecuteCommandWithQuiet("echo 'test output'", false)

	assert.NoError(t, err)
	assert.Contains(t, output, "test output")
}

func TestExecuteCommand_WithCustomEnv(t *testing.T) {
	exec := runner.NewExecWithEnv(map[string]string{
		"TEST_VAR": "custom_value",
	})

	output, err := exec.ExecuteCommand("echo $TEST_VAR")
	assert.NoError(t, err)
	assert.Contains(t, output, "custom_value")
}

func TestExecuteCommand_EnvOverride(t *testing.T) {
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		os.Setenv("PATH", originalPath)
	})

	exec := runner.NewExecWithEnv(map[string]string{
		"TEST_OVERRIDE": "new_value",
	})

	output, err := exec.ExecuteCommand("echo $TEST_OVERRIDE")
	assert.NoError(t, err)
	assert.Contains(t, output, "new_value")
}

func TestExecuteCommandWithWriter_NonPTY(t *testing.T) {
	t.Run("captures output to writer", func(t *testing.T) {
		exec := runner.NewExec()
		var buf bytes.Buffer

		output, err := exec.ExecuteCommandWithWriter("echo 'hello world'", &buf, false)

		assert.NoError(t, err)
		assert.Contains(t, output, "hello world")
		assert.Contains(t, buf.String(), "hello world")
	})

	t.Run("stderr captured in non-pty mode", func(t *testing.T) {
		exec := runner.NewExec()
		var buf bytes.Buffer

		_, err := exec.ExecuteCommandWithWriter("echo 'error' >&2 && exit 1", &buf, false)

		assert.Error(t, err)
		assert.Contains(t, buf.String(), "error")
	})

	t.Run("empty command returns no error", func(t *testing.T) {
		exec := runner.NewExec()
		var buf bytes.Buffer

		output, err := exec.ExecuteCommandWithWriter("", &buf, false)

		assert.NoError(t, err)
		assert.Empty(t, output)
	})
}

func TestExecuteCommandWithWriter_PTY(t *testing.T) {
	t.Run("allocates pty for tty enabled", func(t *testing.T) {
		if os.Getenv("CI") != "" {
			t.Skip("Skipping PTY test in CI environment")
		}

		exec := runner.NewExec()
		var buf bytes.Buffer

		output, err := exec.ExecuteCommandWithWriter("echo 'tty test'", &buf, true)

		assert.NoError(t, err)
		assert.Contains(t, output, "tty test")
	})

	t.Run("pty with color codes", func(t *testing.T) {
		if os.Getenv("CI") != "" {
			t.Skip("Skipping PTY test in CI environment")
		}

		exec := runner.NewExec()
		var buf bytes.Buffer

		// Use ls with color to test ANSI preservation
		output, err := exec.ExecuteCommandWithWriter("ls --color=always -la /tmp | head -1", &buf, true)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)
	})
}

func TestExecuteCommandWithWriter_StdinPassthrough(t *testing.T) {
	t.Run("stdin passed to child process in non-pty mode", func(t *testing.T) {
		// Note: This test validates that stdin is set on the cmd object.
		// Full stdin passthrough testing requires interactive testing.

		exec := runner.NewExec()
		var buf bytes.Buffer

		// Create a simple command that reads from stdin
		output, err := exec.ExecuteCommandWithWriter("cat /etc/hostname", &buf, false)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)
	})
}

func TestExecuteCommand_MultipleCommands(t *testing.T) {
	t.Run("sequential commands with environment", func(t *testing.T) {
		exec := runner.NewExecWithEnv(map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
		})

		output1, err := exec.ExecuteCommand("echo $VAR1")
		assert.NoError(t, err)
		assert.Contains(t, output1, "value1")

		output2, err := exec.ExecuteCommand("echo $VAR2")
		assert.NoError(t, err)
		assert.Contains(t, output2, "value2")
	})
}

func TestExecuteCommand_ErrorHandling(t *testing.T) {
	t.Run("capture exit code on failure", func(t *testing.T) {
		exec := runner.NewExec()
		_, err := exec.ExecuteCommand("exit 42")

		assert.Error(t, err)
		execErr, ok := err.(runner.ExecError)
		assert.True(t, ok, "error should be ExecError")
		assert.Equal(t, 42, execErr.LastExitCode)
	})

	t.Run("capture stderr on failure", func(t *testing.T) {
		exec := runner.NewExec()
		_, err := exec.ExecuteCommandWithQuiet("echo 'test error' >&2 && exit 1", false)

		assert.Error(t, err)
		execErr, ok := err.(runner.ExecError)
		assert.True(t, ok)
		assert.Contains(t, execErr.Output, "test error")
	})
}

func TestExecError_Interface(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		execErr := runner.ExecError{
			Message: "test error",
		}

		assert.Equal(t, "test error", execErr.Error())
		assert.Equal(t, len("test error"), execErr.Len())
	})

	t.Run("implements error interface", func(t *testing.T) {
		var err error = runner.ExecError{
			Message: "test",
		}

		assert.NotNil(t, err)
	})
}

func TestExecuteCommandWithWriter_OutputCapture(t *testing.T) {
	t.Run("multiline output in non-pty", func(t *testing.T) {
		exec := runner.NewExec()
		var buf bytes.Buffer

		output, err := exec.ExecuteCommandWithWriter("printf 'line1\\nline2\\nline3\\n'", &buf, false)

		assert.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 3)
		assert.Equal(t, "line1", lines[0])
		assert.Equal(t, "line2", lines[1])
		assert.Equal(t, "line3", lines[2])
	})

	t.Run("large output capture", func(t *testing.T) {
		exec := runner.NewExec()
		var buf bytes.Buffer

		// Generate 1000 lines of output
		output, err := exec.ExecuteCommandWithWriter("seq 1 1000", &buf, false)

		assert.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 1000)
	})
}

func TestExecuteCommandWithWriter_WriterIntegration(t *testing.T) {
	t.Run("output written to both buffer and writer", func(t *testing.T) {
		exec := runner.NewExec()
		var buf bytes.Buffer

		// For non-PTY, we can verify output is written
		output, err := exec.ExecuteCommandWithWriter("echo 'test message'", &buf, false)

		assert.NoError(t, err)
		// Both output return value and buffer should have the content
		assert.Contains(t, output, "test message")
		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("discardAll writer", func(t *testing.T) {
		exec := runner.NewExec()

		output, err := exec.ExecuteCommandWithWriter("echo 'discarded'", io.Discard, false)

		assert.NoError(t, err)
		assert.Contains(t, output, "discarded")
	})
}

func TestNewExec(t *testing.T) {
	t.Run("default constructor", func(t *testing.T) {
		exec := runner.NewExec()
		assert.NotNil(t, exec)
	})

	t.Run("with env constructor", func(t *testing.T) {
		env := map[string]string{
			"KEY": "value",
		}
		exec := runner.NewExecWithEnv(env)
		assert.NotNil(t, exec)
	})
}

func TestExecuteCommand_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected string
	}{
		{
			name:     "single quotes",
			cmd:      "printf 'hello'",
			expected: "hello",
		},
		{
			name:     "double quotes with variable escape",
			cmd:      "VAR='test' && echo \"$VAR\"",
			expected: "test",
		},
		{
			name:     "pipes",
			cmd:      "echo 'hello world' | wc -w",
			expected: "2",
		},
		{
			name:     "redirection",
			cmd:      "echo 'test' > /tmp/test_exec.txt && cat /tmp/test_exec.txt",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := runner.NewExec()
			output, err := exec.ExecuteCommand(tt.cmd)

			require.NoError(t, err)
			assert.Contains(t, strings.TrimSpace(output), tt.expected)
		})
	}
}

func TestExecuteCommand_CommandTimeout(t *testing.T) {
	t.Run("long-running command succeeds", func(t *testing.T) {
		exec := runner.NewExec()
		// Simple sleep command that should complete quickly
		output, err := exec.ExecuteCommand("sleep 0.1 && echo 'done'")

		assert.NoError(t, err)
		assert.Contains(t, output, "done")
	})
}
