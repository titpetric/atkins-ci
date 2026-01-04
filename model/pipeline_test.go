package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestStepUnmarshalYAML_StringStep tests unmarshalling a simple string step
func TestStepUnmarshalYAML_StringStep(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantRun  string
		wantName string
	}{
		{
			name:     "simple command",
			yaml:     `"echo hello"`,
			wantRun:  "echo hello",
			wantName: "echo hello",
		},
		{
			name:     "command with pipes",
			yaml:     `"ls ./bin/*.test | sh -x"`,
			wantRun:  "ls ./bin/*.test | sh -x",
			wantName: "ls ./bin/*.test | sh -x",
		},
		{
			name:     "complex command",
			yaml:     `"docker compose up -d --wait --remove-orphans"`,
			wantRun:  "docker compose up -d --wait --remove-orphans",
			wantName: "docker compose up -d --wait --remove-orphans",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var step Step
			err := yaml.Unmarshal([]byte(tt.yaml), &step)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if step.Run != tt.wantRun {
				t.Errorf("Run = %q, want %q", step.Run, tt.wantRun)
			}
			if step.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", step.Name, tt.wantName)
			}
		})
	}
}

// TestStepUnmarshalYAML_ObjectStep tests unmarshalling a structured step object
func TestStepUnmarshalYAML_ObjectStep(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantRun    string
		wantName   string
		wantDefer  bool
		wantDetach bool
	}{
		{
			name: "simple object step",
			yaml: `
run: "echo test"
name: "test step"
`,
			wantRun:  "echo test",
			wantName: "test step",
		},
		{
			name: "step with defer",
			yaml: `
defer: true
run: "cleanup cmd"
`,
			wantDefer: true,
			wantRun:   "cleanup cmd",
		},
		{
			name: "step with detach",
			yaml: `
run: "background task"
detach: true
`,
			wantRun:    "background task",
			wantDetach: true,
		},
		{
			name:      "defer-only step",
			yaml:      `run: "docker compose down"\ndefer: true`,
			wantDefer: true,
			wantRun:   "docker compose down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var step Step
			err := yaml.Unmarshal([]byte(tt.yaml), &step)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if step.Run != tt.wantRun {
				t.Errorf("Run = %q, want %q", step.Run, tt.wantRun)
			}
			if step.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", step.Name, tt.wantName)
			}
			if step.Defer != tt.wantDefer {
				t.Errorf("Defer = %v, want %v", step.Defer, tt.wantDefer)
			}
			if step.Detach != tt.wantDetach {
				t.Errorf("Detach = %v, want %v", step.Detach, tt.wantDetach)
			}
		})
	}
}

// TestStepsSliceUnmarshal tests unmarshalling a list of mixed string and object steps
func TestStepsSliceUnmarshal(t *testing.T) {
	yamlStr := `
- echo hello
- run: echo world
  name: named step
- defer: cleanup
- run: test
  detach: true
`

	var steps []Step
	err := yaml.Unmarshal([]byte(yamlStr), &steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 4 {
		t.Fatalf("got %d steps, want 4", len(steps))
	}

	// First step: simple string
	if steps[0].Run != "echo hello" || steps[0].Name != "echo hello" {
		t.Errorf("step[0]: Run=%q Name=%q, want both 'echo hello'", steps[0].Run, steps[0].Name)
	}

	// Second step: object with name
	if steps[1].Run != "echo world" || steps[1].Name != "named step" {
		t.Errorf("step[1]: Run=%q Name=%q, want Run='echo world' Name='named step'", steps[1].Run, steps[1].Name)
	}

	// Third step: defer only
	if steps[2].Defer != "cleanup" || steps[2].Run != "" {
		t.Errorf("step[2]: Defer=%q Run=%q, want Defer='cleanup' Run=''", steps[2].Defer, steps[2].Run)
	}

	// Fourth step: detached
	if steps[3].Run != "test" || !steps[3].Detach {
		t.Errorf("step[3]: Run=%q Detach=%v, want Run='test' Detach=true", steps[3].Run, steps[3].Detach)
	}
}

// TestJobWithStringSteps tests unmarshalling a Job with string steps
func TestJobWithStringSteps(t *testing.T) {
	jobYaml := `
desc: "Test job"
steps:
  - echo hello
  - go test ./...
  - defer: docker compose down
`

	var job Job
	err := yaml.Unmarshal([]byte(jobYaml), &job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Desc != "Test job" {
		t.Errorf("Desc = %q, want 'Test job'", job.Desc)
	}

	if len(job.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(job.Steps))
	}

	// Verify each step
	if job.Steps[0].Run != "echo hello" {
		t.Errorf("steps[0].Run = %q, want 'echo hello'", job.Steps[0].Run)
	}
	if job.Steps[1].Run != "go test ./..." {
		t.Errorf("steps[1].Run = %q, want 'go test ./...'", job.Steps[1].Run)
	}
	if job.Steps[2].Defer != "docker compose down" {
		t.Errorf("steps[2].Defer = %q, want 'docker compose down'", job.Steps[2].Defer)
	}
}
