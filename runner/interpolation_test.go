package runner_test

import (
	"testing"

	"github.com/expr-lang/expr"
	"github.com/stretchr/testify/assert"

	"github.com/titpetric/atkins/runner"
)

func TestInterpolation(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		variables   map[string]any
		env         map[string]string
		expected    string
		expectError bool
	}{
		// Basic variable interpolation
		{
			name:        "simple variable interpolation",
			cmd:         "echo ${{ item }}",
			variables:   map[string]any{"item": "apple"},
			expected:    "echo apple",
			expectError: false,
		},
		{
			name:        "variable without spaces",
			cmd:         "echo ${{item}}",
			variables:   map[string]any{"item": "banana"},
			expected:    "echo banana",
			expectError: false,
		},
		{
			name:        "multiple variable interpolations",
			cmd:         "${{ cmd }} ${{ arg }}",
			variables:   map[string]any{"cmd": "ls", "arg": "-la"},
			expected:    "ls -la",
			expectError: false,
		},

		// Dot notation for nested access
		{
			name: "dot notation with nested object",
			cmd:  "echo ${{ user.name }}",
			variables: map[string]any{
				"user": map[string]any{"name": "alice"},
			},
			expected:    "echo alice",
			expectError: false,
		},
		{
			name: "nested matrix access",
			cmd:  "building ${{ matrix.os }} on ${{ matrix.arch }}",
			variables: map[string]any{
				"matrix": map[string]any{
					"os":   "linux",
					"arch": "amd64",
				},
			},
			expected:    "building linux on amd64",
			expectError: false,
		},

		// Missing variables (should return original)
		{
			name:        "missing variable",
			cmd:         "echo ${{ missing }}",
			variables:   map[string]any{},
			expected:    "echo ${{ missing }}",
			expectError: false,
		},

		// Null coalescing operator ?? (recommended for defaults)
		{
			name:        "null coalescing with first value present",
			cmd:         "echo ${{ a ?? b }}",
			variables:   map[string]any{"a": "value_a", "b": "value_b"},
			expected:    "echo value_a",
			expectError: false,
		},
		{
			name:        "null coalescing with first value nil",
			cmd:         "echo ${{ a ?? b }}",
			variables:   map[string]any{"a": nil, "b": "value_b"},
			expected:    "echo value_b",
			expectError: false,
		},
		{
			name:        "null coalescing with first value missing",
			cmd:         "echo ${{ missing ?? fallback }}",
			variables:   map[string]any{"fallback": "fallback_value"},
			expected:    "echo fallback_value",
			expectError: false,
		},
		{
			name:        "null coalescing with literal default",
			cmd:         "echo ${{ missing ?? 'default' }}",
			variables:   map[string]any{},
			expected:    "echo default",
			expectError: false,
		},
		{
			name: "chained null coalescing",
			cmd:  "echo ${{ a ?? b ?? c }}",
			variables: map[string]any{
				"a": nil,
				"b": nil,
				"c": "final_value",
			},
			expected:    "echo final_value",
			expectError: false,
		},
		{
			name:        "null coalescing with empty string (returns empty string)",
			cmd:         "echo ${{ a ?? 'default' }}",
			variables:   map[string]any{"a": ""},
			expected:    "echo ",
			expectError: false,
		},
		{
			name:        "null coalescing with zero (returns zero)",
			cmd:         "result=${{ count ?? 1 }}",
			variables:   map[string]any{"count": 0},
			expected:    "result=0",
			expectError: false,
		},

		// Environment variables
		{
			name:        "environment variable access",
			cmd:         "echo ${{ HOME }}",
			variables:   map[string]any{},
			env:         map[string]string{"HOME": "/root"},
			expected:    "echo /root",
			expectError: false,
		},
		{
			name:        "environment variable with null coalescing",
			cmd:         "echo ${{ PATH ?? '/bin' }}",
			variables:   map[string]any{},
			env:         map[string]string{"PATH": "/usr/bin"},
			expected:    "echo /usr/bin",
			expectError: false,
		},

		// Complex expressions
		{
			name: "parenthesized expression with null coalescing",
			cmd:  "echo ${{ (a ?? b) ?? c }}",
			variables: map[string]any{
				"a": nil,
				"b": nil,
				"c": "final",
			},
			expected:    "echo final",
			expectError: false,
		},
		{
			name: "mixed operators with dot notation",
			cmd:  "echo ${{ user.name ?? 'anonymous' }}",
			variables: map[string]any{
				"user": map[string]any{"name": nil},
			},
			expected:    "echo anonymous",
			expectError: false,
		},

		// Type conversions
		{
			name:        "numeric value interpolation",
			cmd:         "count=${{ count }}",
			variables:   map[string]any{"count": 42},
			expected:    "count=42",
			expectError: false,
		},
		{
			name:        "boolean value interpolation",
			cmd:         "enabled=${{ enabled }}",
			variables:   map[string]any{"enabled": true},
			expected:    "enabled=true",
			expectError: false,
		},

		// Command substitution with variable interpolation
		{
			name:        "command substitution with variable inside",
			cmd:         "result=$(echo ${{ item }})",
			variables:   map[string]any{"item": "hello"},
			expected:    "result=hello",
			expectError: false,
		},
		{
			name: "command substitution with nested variable",
			cmd:  "result=$(echo ${{ user.name }})",
			variables: map[string]any{
				"user": map[string]any{"name": "alice"},
			},
			expected:    "result=alice",
			expectError: false,
		},
		{
			name:        "command substitution with multiple variables",
			cmd:         "result=$(echo ${{ first }} ${{ second }})",
			variables:   map[string]any{"first": "hello", "second": "world"},
			expected:    "result=hello world",
			expectError: false,
		},
		{
			name:        "command substitution with nested parentheses in jq",
			cmd:         "result=$(echo '[{\"content\":\"TEST\"}]' | jq -r '.[] | select(.content | contains(\"${{ pattern }}\")) | .content')",
			variables:   map[string]any{"pattern": "TEST"},
			expected:    "result=TEST",
			expectError: false,
		},
		{
			name:        "command substitution with complex jq and nested variables",
			cmd:         "result=$(echo '[{\"to\":\"123\",\"time\":\"10:30\"}]' | jq -r '.[] | select(.to == \"${{ number }}\") | .time')",
			variables:   map[string]any{"number": "123"},
			expected:    "result=10:30",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &runner.ExecutionContext{
				Variables: tt.variables,
				Env:       tt.env,
			}

			result, err := runner.InterpolateCommand(tt.cmd, ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNullCoalescingOperator(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		variables map[string]any
		expected  any
	}{
		// ?? operator: null coalescing (use second value if first is nil/missing)
		{
			name:      "?? with defined non-nil value",
			expr:      `a ?? 'default'`,
			variables: map[string]any{"a": "value"},
			expected:  "value",
		},
		{
			name:      "?? with nil value",
			expr:      `a ?? 'default'`,
			variables: map[string]any{"a": nil},
			expected:  "default",
		},
		{
			name:      "?? with empty string (returns empty string, not default)",
			expr:      `a ?? 'default'`,
			variables: map[string]any{"a": ""},
			expected:  "",
		},
		{
			name:      "?? with zero (returns zero, not default)",
			expr:      `a ?? 1`,
			variables: map[string]any{"a": 0},
			expected:  0,
		},
		{
			name:      "?? with false (returns false, not default)",
			expr:      `a ?? 'default'`,
			variables: map[string]any{"a": false},
			expected:  false,
		},
		{
			name: "?? chained with multiple nil values",
			expr: `a ?? b ?? c ?? 'fallback'`,
			variables: map[string]any{
				"a": nil,
				"b": nil,
				"c": nil,
			},
			expected: "fallback",
		},
		{
			name: "?? with first non-nil in chain",
			expr: `a ?? b ?? c`,
			variables: map[string]any{
				"a": nil,
				"b": "middle",
				"c": "last",
			},
			expected: "middle",
		},
		{
			name: "?? with nested object access",
			expr: `user.profile ?? 'no_profile'`,
			variables: map[string]any{
				"user": map[string]any{"profile": nil},
			},
			expected: "no_profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &runner.ExecutionContext{
				Variables: tt.variables,
				Env:       make(map[string]string),
			}

			result, err := evaluateExpr(tt.expr, ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// evaluateExpr is a helper for testing expression evaluation directly
func evaluateExpr(exprStr string, ctx *runner.ExecutionContext) (any, error) {
	env := make(map[string]any)
	for k, v := range ctx.Variables {
		env[k] = v
	}
	for k, v := range ctx.Env {
		env[k] = v
	}

	program, err := expr.Compile(exprStr)
	if err != nil {
		return nil, err
	}
	return expr.Run(program, env)
}
