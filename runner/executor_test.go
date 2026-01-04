package runner_test

import (
	"testing"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/runner"
)

func TestExecuteStepWithForLoop(t *testing.T) {
	tests := []struct {
		name          string
		step          *model.Step
		variables     map[string]interface{}
		expectedCount int
		expectError   bool
	}{
		{
			name: "simple for loop with list",
			step: &model.Step{
				Name: "test step",
				Run:  "echo ${{ item }}",
				For:  "item in fruits",
			},
			variables: map[string]interface{}{
				"fruits": []interface{}{"apple", "banana", "orange"},
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name: "for loop with custom variable name",
			step: &model.Step{
				Name: "test step",
				Run:  "echo ${{ pkg }}",
				For:  "pkg in packages",
			},
			variables: map[string]interface{}{
				"packages": []interface{}{"pkg1", "pkg2"},
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "empty for loop",
			step: &model.Step{
				Name: "test step",
				Run:  "echo ${{ item }}",
				For:  "item in empty",
			},
			variables: map[string]interface{}{
				"empty": []interface{}{},
			},
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &runner.ExecutionContext{
				Variables: tt.variables,
				Step:      tt.step,
				Env:       make(map[string]string),
			}

			iterations, err := runner.ExpandFor(ctx, func(cmd string) (string, error) {
				return "", nil
			})

			if (err != nil) != tt.expectError {
				t.Errorf("ExpandFor error = %v, expectError %v", err, tt.expectError)
				return
			}

			if len(iterations) != tt.expectedCount {
				t.Errorf("ExpandFor returned %d iterations, expected %d", len(iterations), tt.expectedCount)
				return
			}

			// Verify iteration variables are set correctly
			for i, iter := range iterations {
				if iter.Variables == nil {
					t.Errorf("Iteration %d has nil variables", i)
					continue
				}

				// For simple pattern, check if the loop variable is set
				if tt.step.For == "item in fruits" {
					if _, ok := iter.Variables["item"]; !ok {
						t.Errorf("Iteration %d missing 'item' variable", i)
					}
				} else if tt.step.For == "pkg in packages" {
					if _, ok := iter.Variables["pkg"]; !ok {
						t.Errorf("Iteration %d missing 'pkg' variable", i)
					}
				}
			}
		})
	}
}

func TestInterpolationInForLoop(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		variables   map[string]interface{}
		expected    string
		expectError bool
	}{
		{
			name:        "simple variable interpolation",
			cmd:         "echo Fruit: ${{ item }}",
			variables:   map[string]interface{}{"item": "apple"},
			expected:    "echo Fruit: apple",
			expectError: false,
		},
		{
			name:        "multiple variable interpolation",
			cmd:         "echo ${{ key }} = ${{ value }}",
			variables:   map[string]interface{}{"key": "name", "value": "Alice"},
			expected:    "echo name = Alice",
			expectError: false,
		},
		{
			name:        "bash command execution",
			cmd:         "echo $(echo 'hello')",
			variables:   map[string]interface{}{},
			expected:    "echo hello",
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

func TestForLoopStepExecution(t *testing.T) {
	t.Run("for loop with iterator variable", func(t *testing.T) {
		// Executor not needed for this test

		// Create step with for loop
		step := &model.Step{
			Name: "process items",
			Run:  "echo ${{ item }} >> /tmp/test-for-exec.log",
			For:  "item in items",
		}

		// Create execution context with iteration items
		ctx := &runner.ExecutionContext{
			Variables: map[string]interface{}{
				"items": []interface{}{"one", "two", "three"},
			},
			Step:    step,
			Env:     make(map[string]string),
			Results: make(map[string]interface{}),
		}

		// Mock execute function for testing
		mockExecuted := []string{}
		mockExecuteFunc := func(cmd string) (string, error) {
			mockExecuted = append(mockExecuted, cmd)
			return "", nil
		}

		// Expand and verify
		iterations, err := runner.ExpandFor(ctx, mockExecuteFunc)
		if err != nil {
			t.Fatalf("ExpandFor failed: %v", err)
		}

		if len(iterations) != 3 {
			t.Errorf("Expected 3 iterations, got %d", len(iterations))
		}

		// Verify each iteration has the correct variable
		expectedItems := []string{"one", "two", "three"}
		for i, iter := range iterations {
			if iter.Variables["item"] != expectedItems[i] {
				t.Errorf("Iteration %d: expected item=%q, got %q", i, expectedItems[i], iter.Variables["item"])
			}
		}
	})
}
