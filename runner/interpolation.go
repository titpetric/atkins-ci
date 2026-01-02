package runner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/titpetric/atkins-ci/model"
)

var (
	// Matches ${{ variable_name }}
	interpolationRegex = regexp.MustCompile(`\$\{\{\s*([^}]+)\s*\}\}`)
	// Matches $(command)
	commandExecRegex = regexp.MustCompile(`\$\(([^)]+)\)`)
)

// InterpolateString replaces ${{ variable }} with values from context
func InterpolateString(s string, ctx *model.ExecutionContext) (string, error) {
	result := s

	// Handle variable interpolation: ${{ var }}
	result = interpolationRegex.ReplaceAllStringFunc(result, func(match string) string {
		varName := interpolationRegex.FindStringSubmatch(match)[1]
		varName = strings.TrimSpace(varName)

		// Handle dot notation for nested access: matrix.goarch
		if strings.Contains(varName, ".") {
			val := getNestedValue(varName, ctx.Variables)
			if val != nil {
				return fmt.Sprintf("%v", val)
			}
		}

		// Check variables first, then environment
		if val, ok := ctx.Variables[varName]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := ctx.Env[varName]; ok {
			return val
		}
		// Return original if not found
		return match
	})

	// Handle command execution: $(command)
	result = commandExecRegex.ReplaceAllStringFunc(result, func(match string) string {
		cmd := commandExecRegex.FindStringSubmatch(match)[1]
		cmd = strings.TrimSpace(cmd)

		output, err := ExecuteCommand(cmd)
		if err != nil {
			// Return original on error
			return match
		}
		return strings.TrimSpace(output)
	})

	return result, nil
}

// InterpolateMap recursively interpolates all string values in a map
func InterpolateMap(m map[string]interface{}, ctx *model.ExecutionContext) error {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			interpolated, err := InterpolateString(val, ctx)
			if err != nil {
				return err
			}
			m[k] = interpolated
		case map[string]interface{}:
			if err := InterpolateMap(val, ctx); err != nil {
				return err
			}
		case []interface{}:
			for i, item := range val {
				if str, ok := item.(string); ok {
					interpolated, err := InterpolateString(str, ctx)
					if err != nil {
						return err
					}
					val[i] = interpolated
				}
			}
		}
	}
	return nil
}

// InterpolateCommand interpolates a command string
func InterpolateCommand(cmd string, ctx *model.ExecutionContext) (string, error) {
	return InterpolateString(cmd, ctx)
}

// getNestedValue retrieves a nested value using dot notation (e.g., "matrix.goarch")
func getNestedValue(path string, vars map[string]interface{}) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = vars

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}
