package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkingDirectory_ChangesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Chdir(originalDir)
	})

	configPath := filepath.Join(tmpDir, ".atkins.yml")
	err = os.WriteFile(configPath, []byte("name: test\njobs:\n  default:\n    script:\n      - echo hello\n"), 0o644)
	require.NoError(t, err)

	cmd := NewCommand()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cmd.Bind(fs)

	err = fs.Parse([]string{"-w", tmpDir, "-l"})
	require.NoError(t, err)

	err = cmd.Run(t.Context(), fs.Args())
	require.NoError(t, err)

	currentDir, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, tmpDir, currentDir)
}

func TestWorkingDirectory_InvalidDirectory(t *testing.T) {
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Chdir(originalDir)
	})

	cmd := NewCommand()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cmd.Bind(fs)

	err = fs.Parse([]string{"-w", "/nonexistent/path/that/does/not/exist"})
	require.NoError(t, err)

	err = cmd.Run(t.Context(), fs.Args())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to change directory")
}

func TestWorkingDirectory_EmptyIsNoOp(t *testing.T) {
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Chdir(originalDir)
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".atkins.yml")
	err = os.WriteFile(configPath, []byte("name: test\njobs:\n  default:\n    script:\n      - echo hello\n"), 0o644)
	require.NoError(t, err)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	cmd := NewCommand()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cmd.Bind(fs)

	err = fs.Parse([]string{"-l"})
	require.NoError(t, err)

	err = cmd.Run(t.Context(), fs.Args())
	require.NoError(t, err)

	currentDir, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, tmpDir, currentDir)
}
