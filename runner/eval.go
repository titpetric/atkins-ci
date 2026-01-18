package runner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

// EvaluateIf evaluates the If condition using expr-lang.
// Returns true if the condition is met, false if no condition or condition is false.
// Returns error only for invalid expressions.
func EvaluateIf(ctx *ExecutionContext) (bool, error) {
	s := ctx.Step
	if s.If == "" {
		return true, nil // No condition means always execute
	}

	prog, err := expr.Compile(s.If, expr.AllowUndefinedVariables())
	if err != nil {
		return false, fmt.Errorf("failed to compile if expression %q: %w", s.If, err)
	}

	// Build the environment for expression evaluation
	env := make(map[string]any)

	// Add all context variables
	for k, v := range ctx.Variables {
		env[k] = v
	}

	// Add environment variables
	for k, v := range ctx.Env {
		env[k] = v
	}

	// Run the compiled program
	result, err := expr.Run(prog, env)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate if expression %q: %w", s.If, err)
	}

	// Coerce the result to boolean
	switch v := result.(type) {
	case bool:
		return v, nil
	case nil:
		return false, nil
	case string:
		return v != "" && v != "false" && v != "0", nil
	case int, int32, int64:
		return result != 0, nil
	case float32, float64:
		return result != 0.0, nil
	default:
		return true, nil // Non-zero/non-nil values are truthy
	}
}

// ExpandFor expands a for loop into multiple iteration contexts.
// Supports patterns: "item in items" (items is a variable name),
// "(index, item) in items", "(key, value) in items",
// or any of the above with bash expansion: "item in $(ls ./bin/*.test)".
func ExpandFor(ctx *ExecutionContext, executeCommand func(string) (string, error)) ([]IterationContext, error) {
	s := ctx.Step
	if s.For == "" {
		return nil, nil
	}

	// Parse the for loop pattern
	itemsVar, loopVar, indexVar, keyVar, err := parseForPattern(s.For)
	if err != nil {
		return nil, fmt.Errorf("invalid for loop syntax: %w", err)
	}

	// Get the items list
	items, err := getForItems(ctx, itemsVar, executeCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to get items for 'for: %s': %w", s.For, err)
	}

	if len(items) == 0 {
		return []IterationContext{}, nil
	}

	// Build iteration contexts based on the pattern
	var result []IterationContext

	if indexVar != "" || keyVar != "" {
		// (index, item) or (key, value) pattern
		for i, item := range items {
			vars := make(map[string]any)
			for k, v := range ctx.Variables {
				vars[k] = v
			}

			// Check if this is a map for (key, value) iteration
			if mapItem, ok := item.(map[string]any); ok && indexVar != "" && keyVar != "" {
				// Could be either (index, item) with a map item, or (key, value) iteration
				// If items contains only one map, treat as (key, value)
				if len(items) == 1 {
					for k, v := range mapItem {
						vars[indexVar] = k // First var is the key
						vars[keyVar] = v   // Second var is the value
						// Process each key-value pair as a separate iteration
						result = append(result, IterationContext{Variables: copyMap(vars)})
					}
					continue
				}
			}

			if indexVar != "" && keyVar != "" {
				// (index, item) pattern
				vars[indexVar] = i
				vars[keyVar] = item
			} else if keyVar != "" {
				// Fallback for single var with key case
				vars[indexVar] = i
				vars[keyVar] = item
			}
			result = append(result, IterationContext{Variables: vars})
		}
	} else {
		// Simple "item in items" or "name in names" pattern
		// Use the actual loop variable name (loopVar)
		for _, item := range items {
			vars := make(map[string]any)
			for k, v := range ctx.Variables {
				vars[k] = v
			}
			vars[loopVar] = item
			result = append(result, IterationContext{Variables: vars})
		}
	}

	return result, nil
}

// parseForPattern parses for loop patterns and returns (itemsVar, loopVar, indexVar, keyVar, error)
// Patterns: "item in items", "(idx, item) in items", "(key, value) in items"
func parseForPattern(forSpec string) (string, string, string, string, error) {
	forSpec = strings.TrimSpace(forSpec)

	// Match "(var1, var2) in items" or "var in items"
	parenPattern := regexp.MustCompile(`^\s*\(\s*(\w+)\s*,\s*(\w+)\s*\)\s+in\s+(.+)$`)
	simplePattern := regexp.MustCompile(`^\s*(\w+)\s+in\s+(.+)$`)

	if matches := parenPattern.FindStringSubmatch(forSpec); matches != nil {
		// (key, value) or (idx, item)
		// For 2-var pattern, return itemsVar, loopVar="", indexVar, keyVar
		return matches[3], "", matches[1], matches[2], nil
	}

	if matches := simplePattern.FindStringSubmatch(forSpec); matches != nil {
		// item in items
		// For simple pattern: loopVar is the variable name
		return matches[2], matches[1], "", "", nil
	}

	return "", "", "", "", fmt.Errorf("unrecognized for pattern, expected 'item in items' or '(idx, item) in items'")
}

// getForItems retrieves the items list for a for loop
// itemsSpec can be:
//   - A variable name: "items"
//   - A bash command: "$(ls ./bin/*.test)"
//   - An expr-lang expression: `["a", "b", "c"]` or any valid expr returning []any
func getForItems(ctx *ExecutionContext, itemsSpec string, executeCommand func(string) (string, error)) ([]any, error) {
	itemsSpec = strings.TrimSpace(itemsSpec)

	// Check for bash command expansion: $(...)
	if strings.HasPrefix(itemsSpec, "$(") && strings.HasSuffix(itemsSpec, ")") {
		cmd := itemsSpec[2 : len(itemsSpec)-1]

		// Interpolate the command to resolve any variables like ${{ item }}
		interpolated, err := InterpolateString(cmd, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate command %q: %w", cmd, err)
		}

		output, err := executeCommand(interpolated)
		if err != nil {
			return nil, fmt.Errorf("failed to execute command %q: %w", interpolated, err)
		}

		// Split output by newlines
		lines := strings.Split(strings.TrimSpace(output), "\n")
		items := make([]any, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				items = append(items, line)
			}
		}
		return items, nil
	}

	// Check for variable interpolation: ${{ ... }}
	// Extract variable name from ${{ varname }} pattern
	if strings.HasPrefix(itemsSpec, "${{") && strings.HasSuffix(itemsSpec, "}}") {
		exprStr := strings.TrimPrefix(strings.TrimSuffix(itemsSpec, "}}"), "${{")
		exprStr = strings.TrimSpace(exprStr)

		// Evaluate expression (supports dot notation, operators, etc)
		val, err := evaluateExpression(exprStr, ctx)
		if err == nil && val != nil {
			return convertToAnySlice(val)
		}
	}

	// Try evaluating as an expr-lang expression (e.g., array literals like ["a", "b"])
	// This supports inline arrays and other expr constructs
	if val, err := evaluateExpression(itemsSpec, ctx); err == nil && val != nil {
		if items, err := convertToAnySlice(val); err == nil {
			return items, nil
		}
	}

	// Look up in variables
	if val, ok := ctx.Variables[itemsSpec]; ok {
		return convertToAnySlice(val)
	}

	return nil, fmt.Errorf("variable %q not found in context", itemsSpec)
}

// convertToAnySlice converts various types to []any for iteration.
// Supports []any, []string, string (split by newlines), and map[string]any.
func convertToAnySlice(val any) ([]any, error) {
	switch v := val.(type) {
	case []any:
		return v, nil
	case []string:
		items := make([]any, len(v))
		for i, s := range v {
			items[i] = s
		}
		return items, nil
	case string:
		// Split by newlines to support multi-line variables (e.g., from $(command) output)
		lines := strings.Split(strings.TrimSpace(v), "\n")
		items := make([]any, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				items = append(items, line)
			}
		}
		if len(items) > 0 {
			return items, nil
		}
		// If no non-empty lines, return the original string as a single item
		return []any{v}, nil
	case map[string]any:
		// For key-value, return the map as a single item
		return []any{v}, nil
	default:
		return []any{v}, nil
	}
}

// copyMap creates a shallow copy of a map
func copyMap(m map[string]any) map[string]any {
	copy := make(map[string]any)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}
