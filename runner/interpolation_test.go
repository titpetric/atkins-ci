package runner_test

import (
	"testing"

	"github.com/expr-lang/expr"
	"github.com/stretchr/testify/assert"

	"github.com/titpetric/atkins-ci/runner"
)

func TestInterpolation(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		variables   map[string]interface{}
		env         map[string]string
		expected    string
		expectError bool
	}{
		// Basic variable interpolation
		{
			name:        "simple variable interpolation",
			cmd:         "echo ${{ item }}",
			variables:   map[string]interface{}{"item": "apple"},
			expected:    "echo apple",
			expectError: false,
		},
		{
			name:        "variable without spaces",
			cmd:         "echo ${{item}}",
			variables:   map[string]interface{}{"item": "banana"},
			expected:    "echo banana",
			expectError: false,
		},
		{
			name:        "multiple variable interpolations",
			cmd:         "${{ cmd }} ${{ arg }}",
			variables:   map[string]interface{}{"cmd": "ls", "arg": "-la"},
			expected:    "ls -la",
			expectError: false,
		},

		// Dot notation for nested access
		{
			name: "dot notation with nested object",
			cmd:  "echo ${{ user.name }}",
			variables: map[string]interface{}{
				"user": map[string]interface{}{"name": "alice"},
			},
			expected:    "echo alice",
			expectError: false,
		},
		{
			name: "nested matrix access",
			cmd:  "building ${{ matrix.os }} on ${{ matrix.arch }}",
			variables: map[string]interface{}{
				"matrix": map[string]interface{}{
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
			variables:   map[string]interface{}{},
			expected:    "echo ${{ missing }}",
			expectError: false,
		},

		// Null coalescing operator ?? (recommended for defaults)
		{
			name:        "null coalescing with first value present",
			cmd:         "echo ${{ a ?? b }}",
			variables:   map[string]interface{}{"a": "value_a", "b": "value_b"},
			expected:    "echo value_a",
			expectError: false,
		},
		{
			name:        "null coalescing with first value nil",
			cmd:         "echo ${{ a ?? b }}",
			variables:   map[string]interface{}{"a": nil, "b": "value_b"},
			expected:    "echo value_b",
			expectError: false,
		},
		{
			name:        "null coalescing with first value missing",
			cmd:         "echo ${{ missing ?? fallback }}",
			variables:   map[string]interface{}{"fallback": "fallback_value"},
			expected:    "echo fallback_value",
			expectError: false,
		},
		{
			name:        "null coalescing with literal default",
			cmd:         "echo ${{ missing ?? 'default' }}",
			variables:   map[string]interface{}{},
			expected:    "echo default",
			expectError: false,
		},
		{
			name: "chained null coalescing",
			cmd:  "echo ${{ a ?? b ?? c }}",
			variables: map[string]interface{}{
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
			variables:   map[string]interface{}{"a": ""},
			expected:    "echo ",
			expectError: false,
		},
		{
			name:        "null coalescing with zero (returns zero)",
			cmd:         "result=${{ count ?? 1 }}",
			variables:   map[string]interface{}{"count": 0},
			expected:    "result=0",
			expectError: false,
		},

		// Environment variables
		{
			name:        "environment variable access",
			cmd:         "echo ${{ HOME }}",
			variables:   map[string]interface{}{},
			env:         map[string]string{"HOME": "/root"},
			expected:    "echo /root",
			expectError: false,
		},
		{
			name:        "environment variable with null coalescing",
			cmd:         "echo ${{ PATH ?? '/bin' }}",
			variables:   map[string]interface{}{},
			env:         map[string]string{"PATH": "/usr/bin"},
			expected:    "echo /usr/bin",
			expectError: false,
		},

		// Complex expressions
		{
			name: "parenthesized expression with null coalescing",
			cmd:  "echo ${{ (a ?? b) ?? c }}",
			variables: map[string]interface{}{
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
			variables: map[string]interface{}{
				"user": map[string]interface{}{"name": nil},
			},
			expected:    "echo anonymous",
			expectError: false,
		},

		// Type conversions
		{
			name:        "numeric value interpolation",
			cmd:         "count=${{ count }}",
			variables:   map[string]interface{}{"count": 42},
			expected:    "count=42",
			expectError: false,
		},
		{
			name:        "boolean value interpolation",
			cmd:         "enabled=${{ enabled }}",
			variables:   map[string]interface{}{"enabled": true},
			expected:    "enabled=true",
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
		variables map[string]interface{}
		expected  interface{}
	}{
		// ?? operator: null coalescing (use second value if first is nil/missing)
		{
			name:      "?? with defined non-nil value",
			expr:      `a ?? 'default'`,
			variables: map[string]interface{}{"a": "value"},
			expected:  "value",
		},
		{
			name:      "?? with nil value",
			expr:      `a ?? 'default'`,
			variables: map[string]interface{}{"a": nil},
			expected:  "default",
		},
		{
			name:      "?? with empty string (returns empty string, not default)",
			expr:      `a ?? 'default'`,
			variables: map[string]interface{}{"a": ""},
			expected:  "",
		},
		{
			name:      "?? with zero (returns zero, not default)",
			expr:      `a ?? 1`,
			variables: map[string]interface{}{"a": 0},
			expected:  0,
		},
		{
			name:      "?? with false (returns false, not default)",
			expr:      `a ?? 'default'`,
			variables: map[string]interface{}{"a": false},
			expected:  false,
		},
		{
			name: "?? chained with multiple nil values",
			expr: `a ?? b ?? c ?? 'fallback'`,
			variables: map[string]interface{}{
				"a": nil,
				"b": nil,
				"c": nil,
			},
			expected: "fallback",
		},
		{
			name: "?? with first non-nil in chain",
			expr: `a ?? b ?? c`,
			variables: map[string]interface{}{
				"a": nil,
				"b": "middle",
				"c": "last",
			},
			expected: "middle",
		},
		{
			name: "?? with nested object access",
			expr: `user.profile ?? 'no_profile'`,
			variables: map[string]interface{}{
				"user": map[string]interface{}{"profile": nil},
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
func evaluateExpr(exprStr string, ctx *runner.ExecutionContext) (interface{}, error) {
	env := make(map[string]interface{})
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
