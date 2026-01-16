package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/atkins/runner"
)

func TestDiscoverConfig_CurrentDir(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create config in current dir
	configPath := filepath.Join(tmpDir, ".atkins.yml")
	err := os.WriteFile(configPath, []byte("name: test"), 0o644)
	require.NoError(t, err)

	// Test discovery from current dir
	foundPath, foundDir, err := runner.DiscoverConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, configPath, foundPath)
	assert.Equal(t, tmpDir, foundDir)
}

func TestDiscoverConfig_ParentDir(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub", "folder")
	err := os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	// Create config in root
	configPath := filepath.Join(tmpDir, ".atkins.yml")
	err = os.WriteFile(configPath, []byte("name: test"), 0o644)
	require.NoError(t, err)

	// Test discovery from subdir
	foundPath, foundDir, err := runner.DiscoverConfig(subDir)
	require.NoError(t, err)
	assert.Equal(t, configPath, foundPath)
	assert.Equal(t, tmpDir, foundDir)
}

func TestDiscoverConfig_PreferDotFile(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create both configs
	dotConfig := filepath.Join(tmpDir, ".atkins.yml")
	regularConfig := filepath.Join(tmpDir, "atkins.yml")
	err := os.WriteFile(dotConfig, []byte("name: dot"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(regularConfig, []byte("name: regular"), 0o644)
	require.NoError(t, err)

	// Should prefer .atkins.yml
	foundPath, _, err := runner.DiscoverConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, dotConfig, foundPath)
}

func TestDiscoverConfig_FallbackToAtkins(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create only atkins.yml
	configPath := filepath.Join(tmpDir, "atkins.yml")
	err := os.WriteFile(configPath, []byte("name: test"), 0o644)
	require.NoError(t, err)

	// Should find atkins.yml
	foundPath, _, err := runner.DiscoverConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, configPath, foundPath)
}

func TestDiscoverConfig_NotFound(t *testing.T) {
	// Create temp directory with no config
	tmpDir := t.TempDir()

	_, _, err := runner.DiscoverConfig(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no config file found")
}

func TestDiscoverConfig_DeepNesting(t *testing.T) {
	// Create deeply nested structure
	tmpDir := t.TempDir()
	deepDir := filepath.Join(tmpDir, "a", "b", "c", "d", "e")
	err := os.MkdirAll(deepDir, 0o755)
	require.NoError(t, err)

	// Create config at root
	configPath := filepath.Join(tmpDir, ".atkins.yml")
	err = os.WriteFile(configPath, []byte("name: test"), 0o644)
	require.NoError(t, err)

	// Should find it from deep dir
	foundPath, foundDir, err := runner.DiscoverConfig(deepDir)
	require.NoError(t, err)
	assert.Equal(t, configPath, foundPath)
	assert.Equal(t, tmpDir, foundDir)
}
