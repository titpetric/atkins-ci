package runner

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/titpetric/atkins-ci/model"
)

// ProcessEnvDecl processes an EnvDecl and returns a map of environment variables.
// It handles:
// - Manual vars with interpolation ($(...), ${{ ... }})
// - Include files (.env format)
// Vars take precedence over included files.
func ProcessEnvDecl(envDecl *model.EnvDecl, ctx *ExecutionContext) (map[string]string, error) {
	result := make(map[string]string)

	// First, load included files
	if envDecl != nil && envDecl.Include != nil {
		for _, filePath := range envDecl.Include.Files {
			if err := loadEnvFile(filePath, result); err != nil {
				return nil, fmt.Errorf("failed to load env file %q: %w", filePath, err)
			}
		}
	}

	// Then, process and interpolate vars (they override included values)
	if envDecl != nil && envDecl.Vars != nil {
		interpolated, err := interpolateVariables(ctx, envDecl.Vars)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate env vars: %w", err)
		}
		for k, v := range interpolated {
			// Convert to string
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return result, nil
}

// loadEnvFile reads a .env file and populates the env map.
// Format: KEY=VALUE (one per line, # for comments)
func loadEnvFile(filePath string, env map[string]string) error {
	// Interpolate the file path in case it contains variables
	// For now, support simple shell expansion
	expandedPath := os.ExpandEnv(filePath)

	file, err := os.Open(expandedPath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		idx := strings.Index(line, "=")
		if idx == -1 {
			// Skip lines without =
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Handle quoted values
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// MergeEnv merges environment variables from EnvDecl into the execution context.
// Handles both workflow-level, job-level, and step-level env declarations.
func MergeEnv(envDecl *model.EnvDecl, ctx *ExecutionContext) error {
	if envDecl == nil {
		return nil
	}

	processed, err := ProcessEnvDecl(envDecl, ctx)
	if err != nil {
		return err
	}

	// Merge into context
	for k, v := range processed {
		ctx.Env[k] = v
	}

	return nil
}
