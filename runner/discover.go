package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigNames are the default config file names to search for, in order of preference.
var ConfigNames = []string{".atkins.yml", ".atkins.yaml", "atkins.yml", "atkins.yaml"}

// DiscoverConfig searches for a config file starting from the given directory,
// traversing parent directories until a config file is found or root is reached.
// Returns the absolute path to the config file and the directory containing it.
func DiscoverConfig(startDir string) (configPath, configDir string, err error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	dir := absStart
	for {
		for _, name := range ConfigNames {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", "", fmt.Errorf("no config file found (searched for %v)", ConfigNames)
}

// DiscoverConfigFromCwd is a convenience wrapper that starts from the current working directory.
func DiscoverConfigFromCwd() (configPath, configDir string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}
	return DiscoverConfig(cwd)
}
