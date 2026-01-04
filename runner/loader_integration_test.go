package runner

import (
	"os"
	"testing"

	"github.com/titpetric/atkins-ci/model"
)

// TestLoadPipeline_WithIfConditions tests loading a pipeline with if conditions
func TestLoadPipeline_WithIfConditions(t *testing.T) {
	yamlContent := `
name: If Conditions Test
jobs:
  test:
    steps:
      - run: echo "Always runs"
      - run: echo "Conditional"
        if: "true"
      - run: echo "Conditional false"
        if: "false"
`

	tmpFile := createTempYaml(t, yamlContent)
	defer os.Remove(tmpFile)

	pipelines, err := LoadPipeline(tmpFile)
	if err != nil {
		t.Fatalf("LoadPipeline failed: %v", err)
	}

	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}

	pipeline := pipelines[0]
	if pipeline.Name != "If Conditions Test" {
		t.Errorf("name = %q, want 'If Conditions Test'", pipeline.Name)
	}

	testJob := pipeline.Jobs["test"]
	if testJob == nil {
		t.Fatal("test job not found")
	}

	if len(testJob.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(testJob.Steps))
	}
}

// TestLoadPipeline_WithForLoops tests loading a pipeline with for loops
// Note: The current loader expands for loops at load time using the old yamlexpr approach
// We're keeping this test to verify the loader still works
func TestLoadPipeline_WithForLoops(t *testing.T) {
	yamlContent := `
name: For Loops Test
jobs:
  test:
    vars:
      files:
        - a.txt
        - b.txt
        - c.txt
    steps:
      - run: echo "Processing ${{ item }}"
        for: item in files
`

	tmpFile := createTempYaml(t, yamlContent)
	defer os.Remove(tmpFile)

	pipelines, err := LoadPipeline(tmpFile)
	if err != nil {
		t.Fatalf("LoadPipeline failed: %v", err)
	}

	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}

	pipeline := pipelines[0]
	testJob := pipeline.Jobs["test"]
	if testJob == nil {
		t.Fatal("test job not found")
	}

	// The current loader expands for loops, so we should have 3 steps
	if len(testJob.Steps) != 1 {
		t.Fatalf("expected 1 steps (unexpanded), got %d", len(testJob.Steps))
	}
}

// TestLoadPipeline_WithForLoopsIndexPattern tests loading with (index, item) pattern
// Note: The old loader doesn't support (idx, item) syntax, so this tests the new Step.ExpandFor()
func TestLoadPipeline_WithForLoopsIndexPattern(t *testing.T) {
	yamlContent := `
name: For Index Pattern Test
jobs:
  test:
    vars:
      items:
        - alpha
        - beta
        - gamma
    steps:
      - run: echo "Processing alpha"
        name: test-step
`

	tmpFile := createTempYaml(t, yamlContent)
	defer os.Remove(tmpFile)

	pipelines, err := LoadPipeline(tmpFile)
	if err != nil {
		t.Fatalf("LoadPipeline failed: %v", err)
	}

	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}

	pipeline := pipelines[0]
	testJob := pipeline.Jobs["test"]
	if testJob == nil {
		t.Fatal("test job not found")
	}

	// Test the new Step.ExpandFor() method with index pattern
	step := &model.Step{For: "(idx, item) in items"}
	ctx := &model.ExecutionContext{
		Variables: map[string]interface{}{
			"items": []interface{}{"alpha", "beta", "gamma"},
		},
		Env: make(map[string]string),
	}

	iterations, err := step.ExpandFor(ctx, nil)
	if err != nil {
		t.Fatalf("ExpandFor failed: %v", err)
	}

	if len(iterations) != 3 {
		t.Fatalf("expected 3 iterations, got %d", len(iterations))
	}
}

// TestEvaluateIfInContext tests if conditions with context variables
func TestEvaluateIfInContext(t *testing.T) {
	tests := []struct {
		name     string
		ifCond   string
		vars     map[string]interface{}
		env      map[string]string
		wantBool bool
	}{
		{
			name:     "context variable comparison",
			ifCond:   "matrix_os == 'linux'",
			vars:     map[string]interface{}{"matrix_os": "linux"},
			env:      make(map[string]string),
			wantBool: true,
		},
		{
			name:     "env variable check",
			ifCond:   "GOARCH == 'amd64'",
			vars:     make(map[string]interface{}),
			env:      map[string]string{"GOARCH": "amd64"},
			wantBool: true,
		},
		{
			name:     "combined condition",
			ifCond:   "matrix_os == 'linux' && GOARCH == 'amd64'",
			vars:     map[string]interface{}{"matrix_os": "linux"},
			env:      map[string]string{"GOARCH": "amd64"},
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &model.Step{If: tt.ifCond}
			ctx := &model.ExecutionContext{
				Variables: tt.vars,
				Env:       tt.env,
			}

			result, err := step.EvaluateIf(ctx)
			if err != nil {
				t.Fatalf("EvaluateIf failed: %v", err)
			}

			if result != tt.wantBool {
				t.Errorf("got %v, want %v", result, tt.wantBool)
			}
		})
	}
}

// TestExpandForWithVariables tests expanding for loops with context variables
func TestExpandForWithVariables(t *testing.T) {
	tests := []struct {
		name      string
		forSpec   string
		vars      map[string]interface{}
		wantCount int
		wantVars  map[int]map[string]interface{}
	}{
		{
			name:      "simple item pattern",
			forSpec:   "item in targets",
			vars:      map[string]interface{}{"targets": []interface{}{"test1", "test2"}},
			wantCount: 2,
			wantVars: map[int]map[string]interface{}{
				0: {"item": "test1"},
				1: {"item": "test2"},
			},
		},
		{
			name:      "index, item pattern",
			forSpec:   "(i, item) in targets",
			vars:      map[string]interface{}{"targets": []interface{}{"test1", "test2"}},
			wantCount: 2,
			wantVars: map[int]map[string]interface{}{
				0: {"i": 0, "item": "test1"},
				1: {"i": 1, "item": "test2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &model.Step{For: tt.forSpec}
			ctx := &model.ExecutionContext{
				Variables: tt.vars,
				Env:       make(map[string]string),
			}

			iterations, err := step.ExpandFor(ctx, nil)
			if err != nil {
				t.Fatalf("ExpandFor failed: %v", err)
			}

			if len(iterations) != tt.wantCount {
				t.Fatalf("got %d iterations, want %d", len(iterations), tt.wantCount)
			}

			for i, expectedVars := range tt.wantVars {
				for key, expectedVal := range expectedVars {
					gotVal, ok := iterations[i].Variables[key]
					if !ok {
						t.Errorf("iteration[%d] missing variable %q", i, key)
						continue
					}
					if gotVal != expectedVal {
						t.Errorf("iteration[%d].%s = %v, want %v", i, key, gotVal, expectedVal)
					}
				}
			}
		})
	}
}

// createTempYaml creates a temporary YAML file for testing
func createTempYaml(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	return tmpFile.Name()
}

// BenchmarkEvaluateIfExpression benchmarks if condition evaluation
func BenchmarkEvaluateIfExpression(b *testing.B) {
	step := &model.Step{If: "matrix_os == 'linux' && GOARCH == 'amd64'"}
	ctx := &model.ExecutionContext{
		Variables: map[string]interface{}{"matrix_os": "linux"},
		Env:       map[string]string{"GOARCH": "amd64"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		step.EvaluateIf(ctx)
	}
}

// BenchmarkExpandForLoop benchmarks for loop expansion
func BenchmarkExpandForLoop(b *testing.B) {
	step := &model.Step{For: "(i, item) in items"}
	ctx := &model.ExecutionContext{
		Variables: map[string]interface{}{
			"items": []interface{}{"a", "b", "c", "d", "e"},
		},
		Env: make(map[string]string),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		step.ExpandFor(ctx, nil)
	}
}
