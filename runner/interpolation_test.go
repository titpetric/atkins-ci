package runner_test

import (
	"testing"

	"github.com/titpetric/atkins-ci/runner"
)

func TestSingleBraceInterpolation(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		variables   map[string]interface{}
		expected    string
		expectError bool
	}{
		{
			name:        "single brace variable interpolation",
			cmd:         "echo ${{ item }}",
			variables:   map[string]interface{}{"item": "apple"},
			expected:    "echo apple",
			expectError: false,
		},
		{
			name:        "single brace without spaces",
			cmd:         "echo ${{item}}",
			variables:   map[string]interface{}{"item": "banana"},
			expected:    "echo banana",
			expectError: false,
		},
		{
			name:        "multiple single brace interpolations",
			cmd:         "${{ cmd }} ${{ arg }}",
			variables:   map[string]interface{}{"cmd": "ls", "arg": "-la"},
			expected:    "ls -la",
			expectError: false,
		},
		{
			name: "single brace with path notation",
			cmd:  "echo ${{ user.name }}",
			variables: map[string]interface{}{
				"user": map[string]interface{}{"name": "alice"},
			},
			expected:    "echo alice",
			expectError: false,
		},
		{
			name:        "missing variable",
			cmd:         "echo ${{ missing }}",
			variables:   map[string]interface{}{},
			expected:    "echo ${{ missing }}",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &runner.ExecutionContext{
				Variables: tt.variables,
				Env:       make(map[string]string),
			}

			result, err := runner.InterpolateCommand(tt.cmd, ctx)

			if (err != nil) != tt.expectError {
				t.Errorf("InterpolateCommand error = %v, expectError %v", err, tt.expectError)
				return
			}

			if result != tt.expected {
				t.Errorf("InterpolateCommand returned %q, expected %q", result, tt.expected)
			}
		})
	}
}
