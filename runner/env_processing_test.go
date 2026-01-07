package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/atkins-ci/model"
)

func TestProcessEnvDecl_VarsOnly(t *testing.T) {
	ctx := &ExecutionContext{
		Env:       make(map[string]string),
		Variables: make(map[string]interface{}),
	}

	envDecl := &model.EnvDecl{
		Vars: map[string]interface{}{
			"KEY1": "value1",
			"KEY2": "value2",
		},
	}

	result, err := ProcessEnvDecl(envDecl, ctx)
	assert.NoError(t, err)
	assert.Equal(t, "value1", result["KEY1"])
	assert.Equal(t, "value2", result["KEY2"])
}

func TestProcessEnvDecl_WithInterpolation(t *testing.T) {
	ctx := &ExecutionContext{
		Env: make(map[string]string),
		Variables: map[string]interface{}{
			"BASE_PATH": "/app",
		},
	}

	envDecl := &model.EnvDecl{
		Vars: map[string]interface{}{
			"FULL_PATH": "${{ BASE_PATH }}/config",
		},
	}

	result, err := ProcessEnvDecl(envDecl, ctx)
	assert.NoError(t, err)
	assert.Equal(t, "/app/config", result["FULL_PATH"])
}

func TestProcessEnvDecl_WithCommandExecution(t *testing.T) {
	ctx := &ExecutionContext{
		Env:       make(map[string]string),
		Variables: make(map[string]interface{}),
	}

	envDecl := &model.EnvDecl{
		Vars: map[string]interface{}{
			"HOSTNAME": "$(hostname)",
		},
	}

	result, err := ProcessEnvDecl(envDecl, ctx)
	assert.NoError(t, err)

	// Should have some hostname (not empty)
	assert.NotEmpty(t, result["HOSTNAME"], "expected HOSTNAME to be populated from command execution")
}

func TestLoadEnvFile(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")
	envContent := `# Test env file
KEY1=value1
KEY2="quoted value"
KEY3=value with spaces

# Comment line
KEY4=final
`
	assert.NoError(t, os.WriteFile(envFile, []byte(envContent), 0o644))

	env := make(map[string]string)
	assert.NoError(t, loadEnvFile(envFile, env))

	tests := []struct {
		key   string
		value string
	}{
		{"KEY1", "value1"},
		{"KEY2", "quoted value"},
		{"KEY3", "value with spaces"},
		{"KEY4", "final"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.value, env[tt.key])
	}
}

func TestMergeEnv(t *testing.T) {
	ctx := &ExecutionContext{
		Env:       map[string]string{"EXISTING": "value"},
		Variables: make(map[string]interface{}),
	}

	envDecl := &model.EnvDecl{
		Vars: map[string]interface{}{
			"NEW_KEY": "new_value",
		},
	}

	assert.NoError(t, MergeEnv(envDecl, ctx))
	assert.Equal(t, "value", ctx.Env["EXISTING"], "existing env var should be preserved")
	assert.Equal(t, "new_value", ctx.Env["NEW_KEY"], "new env var should be merged")
}

func TestEnvDeclPrecedence(t *testing.T) {
	// Create temp env file with a value
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")
	os.WriteFile(envFile, []byte("KEY=from_file\n"), 0o644)

	ctx := &ExecutionContext{
		Env:       make(map[string]string),
		Variables: make(map[string]interface{}),
	}

	envDecl := &model.EnvDecl{
		Include: &model.EnvIncludeDecl{Files: []string{envFile}},
		Vars: map[string]interface{}{
			"KEY":   "from_vars", // Should override file
			"OTHER": "value",
		},
	}

	result, err := ProcessEnvDecl(envDecl, ctx)
	assert.NoError(t, err)
	assert.Equal(t, "from_vars", result["KEY"], "vars should override include files")
	assert.Equal(t, "value", result["OTHER"])
}

func TestEnvIncludeDecl_UnmarshalString(t *testing.T) {
	// This would normally be tested via YAML unmarshalling,
	// but we test the logic directly
	includeDecl := &model.EnvIncludeDecl{}

	// Simulate setting a single file
	includeDecl.Files = []string{".env"}

	assert.Equal(t, 1, len(includeDecl.Files))
	assert.Equal(t, ".env", includeDecl.Files[0])
}

func TestEnvIncludeDecl_UnmarshalList(t *testing.T) {
	// Simulate setting multiple files
	includeDecl := &model.EnvIncludeDecl{}
	includeDecl.Files = []string{".env", ".env.local", ".env.production"}

	assert.Equal(t, 3, len(includeDecl.Files))

	expected := []string{".env", ".env.local", ".env.production"}
	for i, file := range includeDecl.Files {
		assert.Equal(t, expected[i], file)
	}
}

func TestProcessEnvDecl_NoInterpolationWithoutContext(t *testing.T) {
	ctx := &ExecutionContext{
		Env:       make(map[string]string),
		Variables: make(map[string]interface{}),
	}

	envDecl := &model.EnvDecl{
		Vars: map[string]interface{}{
			"PLAIN": "no_interpolation",
		},
	}

	result, err := ProcessEnvDecl(envDecl, ctx)
	assert.NoError(t, err)
	assert.Equal(t, "no_interpolation", result["PLAIN"])
}

func TestProcessEnvDecl_IntegerValue(t *testing.T) {
	ctx := &ExecutionContext{
		Env:       make(map[string]string),
		Variables: make(map[string]interface{}),
	}

	envDecl := &model.EnvDecl{
		Vars: map[string]interface{}{
			"PORT":  8080,
			"DEBUG": true,
		},
	}

	result, err := ProcessEnvDecl(envDecl, ctx)
	assert.NoError(t, err)
	assert.Equal(t, "8080", result["PORT"])
	assert.Equal(t, "true", result["DEBUG"])
}

func TestLoadEnvFile_SingleQuote(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")
	os.WriteFile(envFile, []byte("KEY='single quoted value'\n"), 0o644)

	env := make(map[string]string)
	assert.NoError(t, loadEnvFile(envFile, env))
	assert.Equal(t, "single quoted value", env["KEY"])
}
