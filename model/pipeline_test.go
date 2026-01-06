package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"
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
			assert.NoError(t, err)
			assert.Equal(t, tt.wantRun, step.Run)
			assert.Equal(t, tt.wantName, step.Name)
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
		wantError  bool
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
defer: cleanup cmd
run: "main task"
`,
			wantError: true,
		},
		{
			name: "step with deferred",
			yaml: `
deferred: true
run: "main task"
`,
			wantDefer: true,
			wantRun:   "main task",
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
			name: "defer-only step with referred",
			yaml: `
run: "docker compose down"
deferred: true
`,
			wantDefer: true,
			wantRun:   "docker compose down",
		},
		{
			name: "defer-only step",
			yaml: `
run: "docker compose down"
defer: cleanup
`,
			wantError: true,
		},
	}

	for index, tt := range tests {
		t.Run(fmt.Sprintf("%d/%s", index, tt.name), func(t *testing.T) {
			var step Step
			err := yaml.Unmarshal([]byte(tt.yaml), &step)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantRun, step.Run)
			assert.Equal(t, tt.wantName, step.Name)
			assert.Equal(t, tt.wantDefer, step.IsDeferred())
			assert.Equal(t, tt.wantDetach, step.Detach)
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
	assert.NoError(t, err)
	assert.Len(t, steps, 4)

	// First step: simple string
	assert.Equal(t, "echo hello", steps[0].Run)
	assert.Equal(t, "echo hello", steps[0].Name)

	// Second step: object with name
	assert.Equal(t, "echo world", steps[1].Run)
	assert.Equal(t, "named step", steps[1].Name)

	// Fourth step: detached
	assert.Equal(t, "test", steps[3].Run)
	assert.True(t, steps[3].Detach)
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
	assert.NoError(t, err)
	assert.Equal(t, "Test job", job.Desc)
	assert.Len(t, job.Steps, 3)

	// Verify each step
	assert.Equal(t, "echo hello", job.Steps[0].Run)
	assert.Equal(t, "go test ./...", job.Steps[1].Run)
}
