package runner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/atkins/model"
	"github.com/titpetric/atkins/runner"
)

func TestExecuteStepWithForLoop(t *testing.T) {
	tests := []struct {
		name          string
		step          *model.Step
		variables     map[string]any
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
			variables: map[string]any{
				"fruits": []any{"apple", "banana", "orange"},
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
			variables: map[string]any{
				"packages": []any{"pkg1", "pkg2"},
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
			variables: map[string]any{
				"empty": []any{},
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
				assert.Fail(t, "ExpandFor error mismatch", "error = %v, expectError %v", err, tt.expectError)
				return
			}

			assert.Equal(t, tt.expectedCount, len(iterations), "expected %d iterations", tt.expectedCount)

			// Verify iteration variables are set correctly
			for i, iter := range iterations {
				assert.NotNil(t, iter.Variables, "Iteration %d has nil variables", i)

				// For simple pattern, check if the loop variable is set
				if tt.step.For == "item in fruits" {
					_, ok := iter.Variables["item"]
					assert.True(t, ok, "Iteration %d missing 'item' variable", i)
				} else if tt.step.For == "pkg in packages" {
					_, ok := iter.Variables["pkg"]
					assert.True(t, ok, "Iteration %d missing 'pkg' variable", i)
				}
			}
		})
	}
}

func TestInterpolationInForLoop(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		variables   map[string]any
		expected    string
		expectError bool
	}{
		{
			name:        "simple variable interpolation",
			cmd:         "echo Fruit: ${{ item }}",
			variables:   map[string]any{"item": "apple"},
			expected:    "echo Fruit: apple",
			expectError: false,
		},
		{
			name:        "multiple variable interpolation",
			cmd:         "echo ${{ key }} = ${{ value }}",
			variables:   map[string]any{"key": "name", "value": "Alice"},
			expected:    "echo name = Alice",
			expectError: false,
		},
		{
			name:        "bash command execution",
			cmd:         "echo $(echo 'hello')",
			variables:   map[string]any{},
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

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expected, result)
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
			Variables: map[string]any{
				"items": []any{"one", "two", "three"},
			},
			Step:    step,
			Env:     make(map[string]string),
			Results: make(map[string]any),
		}

		// Mock execute function for testing
		mockExecuted := []string{}
		mockExecuteFunc := func(cmd string) (string, error) {
			mockExecuted = append(mockExecuted, cmd)
			return "", nil
		}

		// Expand and verify
		iterations, err := runner.ExpandFor(ctx, mockExecuteFunc)
		assert.NoError(t, err)

		assert.Equal(t, 3, len(iterations))

		// Verify each iteration has the correct variable
		expectedItems := []string{"one", "two", "three"}
		for i, iter := range iterations {
			assert.Equal(t, expectedItems[i], iter.Variables["item"], "Iteration %d item mismatch", i)
		}
	})
}

func TestValidateJobRequirements(t *testing.T) {
	tests := []struct {
		name      string
		job       *model.Job
		variables map[string]any
		expectErr bool
		errMsg    string
	}{
		{
			name: "no requirements",
			job: &model.Job{
				Name:     "test_job",
				Requires: []string{},
			},
			variables: map[string]any{},
			expectErr: false,
		},
		{
			name: "requirements satisfied",
			job: &model.Job{
				Name:     "build_component",
				Requires: []string{"component"},
			},
			variables: map[string]any{
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
			variables: map[string]any{},
			expectErr: true,
			errMsg:    "requires variables [component] but missing: [component]",
		},
		{
			name: "multiple requirements, some missing",
			job: &model.Job{
				Name:     "deploy_service",
				Requires: []string{"service", "version", "env"},
			},
			variables: map[string]any{
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
			variables: map[string]any{
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

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStepWithMultipleCmds(t *testing.T) {
	t.Run("step with multiple cmds should display count", func(t *testing.T) {
		step := &model.Step{
			Name: "test multiple commands",
			Cmds: []string{"echo cmd1", "echo cmd2", "echo cmd3"},
		}

		// String() should show count, not expanded commands
		result := step.String()
		assert.Equal(t, "cmds: <3 commands>", result)
		assert.NotContains(t, result, "&&")
		assert.NotContains(t, result, "echo cmd1")
	})
}

func TestIsEchoCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected bool
	}{
		{
			name:     "simple echo command",
			cmd:      "echo hello",
			expected: true,
		},
		{
			name:     "echo with quoted string",
			cmd:      "echo 'hello world'",
			expected: true,
		},
		{
			name:     "echo with variable",
			cmd:      "echo ${{ name }}",
			expected: true,
		},
		{
			name:     "echo with leading spaces",
			cmd:      "   echo test",
			expected: true,
		},
		{
			name:     "echo with multiline",
			cmd:      "echo hello\necho world",
			expected: false,
		},
		{
			name:     "non-echo command",
			cmd:      "make build",
			expected: false,
		},
		{
			name:     "command with echo in middle",
			cmd:      "cmd | echo test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runner.IsEchoCommand(tt.cmd)
			assert.Equal(t, tt.expected, result)
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
			Variables: map[string]any{
				"components": []any{"src/main", "src/utils", "tests/"},
			},
			Step: step,
			Env:  make(map[string]string),
		}

		// Expand the for loop
		iterations, err := runner.ExpandFor(ctx, func(cmd string) (string, error) {
			return "", nil
		})
		assert.NoError(t, err)

		assert.Equal(t, 3, len(iterations))

		// Verify each iteration has the component variable
		expectedComponents := []string{"src/main", "src/utils", "tests/"}
		for i, iter := range iterations {
			assert.Equal(t, expectedComponents[i], iter.Variables["component"], "Iteration %d component mismatch", i)
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
			Variables: map[string]any{
				"component": "src/main",
			},
		}

		// Should pass validation
		err := runner.ValidateJobRequirements(task, ctx)
		assert.NoError(t, err)
	})

	t.Run("task requires variable missing from for loop context", func(t *testing.T) {
		// Create a task that requires a variable
		task := &model.Job{
			Name:     "build_component",
			Requires: []string{"component"},
		}

		// Iteration context without the required variable
		ctx := &runner.ExecutionContext{
			Variables: map[string]any{},
		}

		// Should fail validation
		err := runner.ValidateJobRequirements(task, ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "component")
	})
}

func TestJobVariablesInForLoop(t *testing.T) {
	t.Run("job variable available in for loop expansion", func(t *testing.T) {
		// Simulate a job with variables and a for loop step
		// Like: vars: { testBinaries: "$(ls ./bin/*.test)" }
		//       steps: [{ for: "item in testBinaries" }]

		step := &model.Step{
			Name: "process test binaries",
			Task: "test:detail",
			For:  "item in testBinaries",
		}

		// Job variables are merged into context BEFORE step execution
		ctx := &runner.ExecutionContext{
			Variables: map[string]any{
				"testBinaries": "runner.test\nmodel.test\ntreeview.test",
			},
			Step: step,
			Env:  make(map[string]string),
		}

		// Expand the for loop - should find testBinaries variable
		iterations, err := runner.ExpandFor(ctx, func(cmd string) (string, error) {
			return "", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, len(iterations))

		expectedBinaries := []string{"runner.test", "model.test", "treeview.test"}
		for i, iter := range iterations {
			assert.Equal(t, expectedBinaries[i], iter.Variables["item"], "Iteration %d item mismatch", i)
		}
	})

	t.Run("job variables merged from Decl", func(t *testing.T) {
		// Test that job variables in Decl are properly merged
		job := &model.Job{
			Name: "test_job",
			Decl: &model.Decl{
				Vars: map[string]any{
					"testBinaries": "file1.test\nfile2.test",
				},
			},
		}

		ctx := &runner.ExecutionContext{
			Variables: make(map[string]any),
			Env:       make(map[string]string),
		}

		// Merge job variables
		err := runner.MergeVariables(job.Decl, ctx)
		assert.NoError(t, err)

		// Check that testBinaries is in Variables
		assert.NotNil(t, ctx.Variables["testBinaries"], "testBinaries should be in context variables")
		assert.Equal(t, "file1.test\nfile2.test", ctx.Variables["testBinaries"])
	})

	t.Run("actual pipeline with for loop accessing job variable", func(t *testing.T) {
		// Test the actual scenario from atkins.yml
		// test:run job has vars: { testBinaries: ... }
		// and a step: { for: "item in testBinaries", task: "test:detail" }

		testBinariesValue := "runner.test\nmodel.test\ntreeview.test"

		// Simulate the test:run job
		job := &model.Job{
			Name: "test:run",
			Decl: &model.Decl{
				Vars: map[string]any{
					"testBinaries": testBinariesValue,
				},
			},
			Steps: []*model.Step{
				{
					Name: "process items",
					Task: "test:detail",
					For:  "item in testBinaries",
				},
			},
		}

		// Simulate ExecuteJob flow
		ctx := &runner.ExecutionContext{
			Variables: make(map[string]any),
			Env:       make(map[string]string),
			Job:       job,
		}

		// Step 1: Merge job variables (from ExecuteJob line 119)
		err := runner.MergeVariables(job.Decl, ctx)
		assert.NoError(t, err)
		assert.Equal(t, testBinariesValue, ctx.Variables["testBinaries"], "testBinaries should be merged")

		// Step 2: Simulate executeStep which calls Copy() then calls executeStepWithForLoop
		stepCtx := ctx.Copy() // This should copy variables
		stepCtx.Step = job.Steps[0]
		stepCtx.Env = make(map[string]string)

		// Verify variables are copied correctly
		assert.Equal(t, testBinariesValue, stepCtx.Variables["testBinaries"], "stepCtx should have testBinaries after Copy()")

		// Step 3: Call ExpandFor - it should find testBinaries
		iterations, err := runner.ExpandFor(stepCtx, func(cmd string) (string, error) {
			return "", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, len(iterations), "Should have 3 iterations")
	})
}
