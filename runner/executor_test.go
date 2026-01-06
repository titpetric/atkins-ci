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

func TestValidateJobRequirements(t *testing.T) {
	tests := []struct {
		name      string
		job       *model.Job
		variables map[string]interface{}
		expectErr bool
		errMsg    string
	}{
		{
			name: "no requirements",
			job: &model.Job{
				Name:     "test_job",
				Requires: []string{},
			},
			variables: map[string]interface{}{},
			expectErr: false,
		},
		{
			name: "requirements satisfied",
			job: &model.Job{
				Name:     "build_component",
				Requires: []string{"component"},
			},
			variables: map[string]interface{}{
				"component": "src/main",
			},
			expectErr: false,
		},
		{
			name: "single requirement missing",
			job: &model.Job{
				Name:     "build_component",
				Requires: []string{"component"},
			},
			variables: map[string]interface{}{},
			expectErr: true,
			errMsg:    "requires variables [component] but missing: [component]",
		},
		{
			name: "multiple requirements, some missing",
			job: &model.Job{
				Name:     "deploy_service",
				Requires: []string{"service", "version", "env"},
			},
			variables: map[string]interface{}{
				"service": "api",
				"version": "1.0.0",
			},
			expectErr: true,
			errMsg:    "requires variables [service version env] but missing: [env]",
		},
		{
			name: "all requirements present",
			job: &model.Job{
				Name:     "deploy_service",
				Requires: []string{"service", "version", "env"},
			},
			variables: map[string]interface{}{
				"service": "api",
				"version": "1.0.0",
				"env":     "prod",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &runner.ExecutionContext{
				Variables: tt.variables,
			}
			// Ensure job name is set (already set by test case)

			err := runner.ValidateJobRequirements(tt.job, ctx)

			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateJobRequirements error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateJobRequirements error message = %q, expected to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestTaskInvocationWithForLoop(t *testing.T) {
	t.Run("expand for loop with task variables", func(t *testing.T) {
		// Create a step that invokes a task with a for loop
		step := &model.Step{
			Name: "build all components",
			Task: "build_component",
			For:  "component in components",
		}

		// Create execution context
		ctx := &runner.ExecutionContext{
			Variables: map[string]interface{}{
				"components": []interface{}{"src/main", "src/utils", "tests/"},
			},
			Step: step,
			Env:  make(map[string]string),
		}

		// Expand the for loop
		iterations, err := runner.ExpandFor(ctx, func(cmd string) (string, error) {
			return "", nil
		})
		if err != nil {
			t.Fatalf("ExpandFor failed: %v", err)
		}

		if len(iterations) != 3 {
			t.Errorf("Expected 3 iterations, got %d", len(iterations))
		}

		// Verify each iteration has the component variable
		expectedComponents := []string{"src/main", "src/utils", "tests/"}
		for i, iter := range iterations {
			if iter.Variables["component"] != expectedComponents[i] {
				t.Errorf("Iteration %d: expected component=%q, got %q", i, expectedComponents[i], iter.Variables["component"])
			}
		}
	})

	t.Run("task requires variable from for loop", func(t *testing.T) {
		// Create a task that requires the loop variable
		task := &model.Job{
			Name:     "build_component",
			Requires: []string{"component"},
		}

		// Simulate iteration context with loop variable
		ctx := &runner.ExecutionContext{
			Variables: map[string]interface{}{
				"component": "src/main",
			},
		}

		// Should pass validation
		err := runner.ValidateJobRequirements(task, ctx)
		if err != nil {
			t.Errorf("ValidateJobRequirements failed: %v", err)
		}
	})

	t.Run("task requires variable missing from for loop context", func(t *testing.T) {
		// Create a task that requires a variable
		task := &model.Job{
			Name:     "build_component",
			Requires: []string{"component"},
		}

		// Iteration context without the required variable
		ctx := &runner.ExecutionContext{
			Variables: map[string]interface{}{},
		}

		// Should fail validation
		err := runner.ValidateJobRequirements(task, ctx)
		if err == nil {
			t.Errorf("Expected ValidateJobRequirements to fail, but it passed")
		}

		if !contains(err.Error(), "component") {
			t.Errorf("Expected error to mention 'component', got: %v", err)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
