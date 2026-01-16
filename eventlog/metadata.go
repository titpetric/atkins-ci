package eventlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CaptureGitInfo captures git repository information.
func CaptureGitInfo() *GitInfo {
	info := &GitInfo{}

	// Get commit
	if out, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		info.Commit = strings.TrimSpace(string(out))
	}

	// Get branch
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		info.Branch = strings.TrimSpace(string(out))
	}

	// Get remote URL
	if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
		info.RemoteURL = strings.TrimSpace(string(out))
		info.Repository = extractRepoFromURL(info.RemoteURL)
	}

	// Return nil if no git info was captured
	if info.Commit == "" && info.Branch == "" && info.RemoteURL == "" {
		return nil
	}

	return info
}

// extractRepoFromURL extracts repository name from a git URL.
func extractRepoFromURL(url string) string {
	// Handle SSH URLs: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
	}

	// Handle HTTPS URLs: https://github.com/owner/repo.git
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, ".git")

	return url
}

// CaptureModulePath captures the Go module path from go.mod if present.
func CaptureModulePath() string {
	// Look for go.mod in current directory and parents
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		modPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modPath); err == nil {
			// Parse module line
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module"))
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}
