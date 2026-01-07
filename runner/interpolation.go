package runner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

var (
	// Matches ${{ variable_name }}
	interpolationRegex = regexp.MustCompile(`\$\{\{\s*([^}]+?)\s*\}\}`)

	// Matches $(command)
	commandExecRegex = regexp.MustCompile(`\$\(([^)]+)\)`)
)

// InterpolateString replaces ${{ expression }} with values from context.
// Supports variable interpolation, dot notation, and expr expressions with ?? and || operators.
func InterpolateString(s string, ctx *ExecutionContext) (string, error) {
	result := s

	// Handle variable interpolation: ${{ expression }}
	result = interpolationRegex.ReplaceAllStringFunc(result, func(match string) string {
		exprStr := interpolationRegex.FindStringSubmatch(match)[1]
		exprStr = strings.TrimSpace(exprStr)

		// Evaluate expression using expr-lang
		val, err := evaluateExpression(exprStr, ctx)
		if err != nil {
			// If expression evaluation fails, return original match
			return match
		}

		// Convert result to string
		if val != nil {
			return fmt.Sprintf("%v", val)
		}

		// Return original if result is nil
		return match
	})

	// Handle command execution: $(command)
	// We need to track errors since ReplaceAllStringFunc can't return errors
	var cmdErr error
	result = commandExecRegex.ReplaceAllStringFunc(result, func(match string) string {
		if cmdErr != nil {
			return match
		}
		cmd := commandExecRegex.FindStringSubmatch(match)[1]
		cmd = strings.TrimSpace(cmd)

		// Execute with context env variables
		exec := NewExecWithEnv(ctx.Env)
		output, err := exec.ExecuteCommand(cmd)
		if err != nil {
			// Capture error and return original string
			cmdErr = fmt.Errorf("command execution failed: %w", err)
			return match
		}
		return strings.TrimSpace(output)
	})

	if cmdErr != nil {
		return "", cmdErr
	}

	return result, nil
}

// InterpolateMap recursively interpolates all string values in a map.
func InterpolateMap(ctx *ExecutionContext, m map[string]interface{}) error {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			interpolated, err := InterpolateString(val, ctx)
			if err != nil {
				return err
			}
			m[k] = interpolated
		case map[string]interface{}:
			if err := InterpolateMap(ctx, val); err != nil {
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

// InterpolateCommand interpolates a command string.
func InterpolateCommand(cmd string, ctx *ExecutionContext) (string, error) {
	return InterpolateString(cmd, ctx)
}

// evaluateExpression evaluates an expr expression with access to variables and environment.
// Uses expr-lang/expr for evaluation with support for:
//   - Simple variable access: varName
//   - Dot notation: user.name
//   - Null coalescing (RECOMMENDED): var ?? default
//   - Returns second value only if first is nil/missing
//   - Empty strings, false, 0 are valid and won't trigger default
//   - Complex expressions: (var1 ?? var2) ?? 'fallback'
//
// Note: The ?? (null coalescing) operator is the preferred pattern for defaults
// since it explicitly handles nil/missing values without side effects on falsy values.
func evaluateExpression(exprStr string, ctx *ExecutionContext) (interface{}, error) {
	// Merge variables and environment into a single map for expr evaluation
	env := make(map[string]interface{})
	for k, v := range ctx.Variables {
		env[k] = v
	}
	for k, v := range ctx.Env {
		env[k] = v
	}

	// Compile and evaluate the expression
	program, err := expr.Compile(exprStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return result, nil
}
