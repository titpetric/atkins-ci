package model_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"

	"github.com/titpetric/atkins/model"
)

// TestJobUnmarshalYAML_WithVars tests that Job.Decl.Vars are properly decoded.
func TestJobUnmarshalYAML_WithVars(t *testing.T) {
	yamlContent := `
name: test:run
vars:
  testBinaries: "file1.test\nfile2.test"
  count: 42
steps:
  - run: echo test
`

	var job model.Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	assert.NoError(t, err)

	// Check that Decl is not nil
	assert.NotNil(t, job.Decl)

	// Check that Vars are loaded
	assert.NotNil(t, job.Decl.Vars)
	assert.Equal(t, "file1.test\nfile2.test", job.Decl.Vars["testBinaries"])
	assert.Equal(t, 42, job.Decl.Vars["count"])
}

// TestStepUnmarshalYAML_WithVars tests that Step.Decl.Vars are properly decoded.
func TestStepUnmarshalYAML_WithVars(t *testing.T) {
	yamlContent := `
name: test step
run: echo test
vars:
  myVar: value123
  count: 42
`

	var step model.Step
	err := yaml.Unmarshal([]byte(yamlContent), &step)
	assert.NoError(t, err)

	// Check that Decl is not nil
	assert.NotNil(t, step.Decl)

	// Check that Vars are loaded
	assert.NotNil(t, step.Decl.Vars)
	assert.Equal(t, "value123", step.Decl.Vars["myVar"])
	assert.Equal(t, 42, step.Decl.Vars["count"])
}

// TestPipelineUnmarshalYAML_WithVars tests that Pipeline.Decl.Vars are properly decoded.
func TestPipelineUnmarshalYAML_WithVars(t *testing.T) {
	yamlContent := `
name: Test Pipeline
vars:
  globalVar: global_value
  version: "1.0.0"
jobs:
  test:
    steps:
      - run: echo test
`

	var pipeline model.Pipeline
	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	assert.NoError(t, err)

	// Check that Decl is not nil
	assert.NotNil(t, pipeline.Decl)

	// Check that Vars are loaded
	assert.NotNil(t, pipeline.Decl.Vars)
	assert.Equal(t, "global_value", pipeline.Decl.Vars["globalVar"])
	assert.Equal(t, "1.0.0", pipeline.Decl.Vars["version"])
}

// TestJobUnmarshalYAML_FullDepthDecoding tests full decoding of vars and include in Decl.
func TestJobUnmarshalYAML_FullDepthDecoding(t *testing.T) {
	// Create a temporary include file
	tmpFile, err := os.CreateTemp("", "test-vars-*.yml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write vars to include file
	_, err = tmpFile.WriteString(`
includedVar: included_value
nestedObject:
  key: nested_key_value
`)
	assert.NoError(t, err)
	tmpFile.Close()

	yamlContent := `
name: test:job
vars:
  localVar: local_value
  localCount: 123
include:
  - ` + tmpFile.Name() + `
steps:
  - run: echo test
`

	var job model.Job
	err = yaml.Unmarshal([]byte(yamlContent), &job)
	assert.NoError(t, err)

	// Check vars (at Decl level)
	assert.NotNil(t, job.Decl.Vars)
	assert.Equal(t, "local_value", job.Decl.Vars["localVar"])
	assert.Equal(t, 123, job.Decl.Vars["localCount"])

	// Check include (at Decl level)
	assert.NotNil(t, job.Decl.Include)
	assert.Len(t, job.Decl.Include.Files, 1)
	assert.Equal(t, tmpFile.Name(), job.Decl.Include.Files[0])
}

// TestStepUnmarshalYAML_WithInclude tests that Step.Decl.Include is properly decoded.
func TestStepUnmarshalYAML_WithInclude(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-step-vars-*.yml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(`stepVar: step_var_value`)
	assert.NoError(t, err)
	tmpFile.Close()

	yamlContent := `
name: test step
run: echo test
include:
  - ` + tmpFile.Name()

	var step model.Step
	err = yaml.Unmarshal([]byte(yamlContent), &step)
	assert.NoError(t, err)

	// Check that Decl is not nil
	assert.NotNil(t, step.Decl)

	// Check that Include is loaded
	assert.NotNil(t, step.Decl.Include)
	assert.Len(t, step.Decl.Include.Files, 1)
	assert.Equal(t, tmpFile.Name(), step.Decl.Include.Files[0])
}

// TestNestedJobInheritance tests that nested jobs (with ':' in name) properly decode Decl.
func TestNestedJobInheritance(t *testing.T) {
	yamlContent := `
vars:
  nestedVar: nested_value
desc: "A nested job"
steps:
   - run: echo nested
`

	var job model.Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	assert.NoError(t, err)

	// Set the name to simulate a nested job
	job.Name = "test:nested:job"

	// Check Decl initialization
	assert.NotNil(t, job.Decl)
	assert.NotNil(t, job.Decl.Vars)
	assert.Equal(t, "nested_value", job.Decl.Vars["nestedVar"])

	// Check that IsRootLevel correctly identifies nested jobs
	assert.False(t, job.IsRootLevel())

	// Check that root level jobs are identified correctly
	job.Name = "rootjob"
	assert.True(t, job.IsRootLevel())
}

// TestStepUnmarshalYAML_WithEnv tests that Step.Decl.Env is properly decoded.
func TestStepUnmarshalYAML_WithEnv(t *testing.T) {
	yamlContent := `
name: test step
run: echo test
env:
  vars:
    MY_VAR: myvalue
    ANOTHER_VAR: another_value
`

	var step model.Step
	err := yaml.Unmarshal([]byte(yamlContent), &step)
	assert.NoError(t, err)

	// Check that Decl is not nil
	assert.NotNil(t, step.Decl)

	// Check that Env is loaded
	assert.NotNil(t, step.Decl.Env)
	assert.NotNil(t, step.Decl.Env.Vars)
	assert.Equal(t, "myvalue", step.Decl.Env.Vars["MY_VAR"])
	assert.Equal(t, "another_value", step.Decl.Env.Vars["ANOTHER_VAR"])
}

// TestJobUnmarshalYAML_WithEnv tests that Job.Decl.Env is properly decoded.
func TestJobUnmarshalYAML_WithEnv(t *testing.T) {
	yamlContent := `
name: test:job
env:
  vars:
    JOB_ENV_VAR: job_env_value
    GOOS: linux
    GOARCH: amd64
steps:
  - run: echo test
`

	var job model.Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	assert.NoError(t, err)

	// Check that Decl is not nil
	assert.NotNil(t, job.Decl)

	// Check that Env is loaded
	assert.NotNil(t, job.Decl.Env)
	assert.NotNil(t, job.Decl.Env.Vars)
	assert.Equal(t, "job_env_value", job.Decl.Env.Vars["JOB_ENV_VAR"])
	assert.Equal(t, "linux", job.Decl.Env.Vars["GOOS"])
	assert.Equal(t, "amd64", job.Decl.Env.Vars["GOARCH"])
}
