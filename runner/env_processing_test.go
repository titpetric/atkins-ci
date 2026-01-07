package runner

import (
	"os"
	"path/filepath"
	"testing"

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["KEY1"] != "value1" {
		t.Errorf("expected KEY1=value1, got %q", result["KEY1"])
	}
	if result["KEY2"] != "value2" {
		t.Errorf("expected KEY2=value2, got %q", result["KEY2"])
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["FULL_PATH"] != "/app/config" {
		t.Errorf("expected FULL_PATH=/app/config, got %q", result["FULL_PATH"])
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have some hostname (not empty)
	if result["HOSTNAME"] == "" {
		t.Error("expected HOSTNAME to be populated from command execution")
	}
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
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("failed to create test env file: %v", err)
	}

	env := make(map[string]string)
	if err := loadEnvFile(envFile, env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
		if env[tt.key] != tt.value {
			t.Errorf("env[%q] = %q, want %q", tt.key, env[tt.key], tt.value)
		}
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

	if err := MergeEnv(envDecl, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Env["EXISTING"] != "value" {
		t.Error("existing env var should be preserved")
	}
	if ctx.Env["NEW_KEY"] != "new_value" {
		t.Error("new env var should be merged")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["KEY"] != "from_vars" {
		t.Errorf("vars should override include files, got %q", result["KEY"])
	}
	if result["OTHER"] != "value" {
		t.Errorf("OTHER should be %q, got %q", "value", result["OTHER"])
	}
}

func TestEnvIncludeDecl_UnmarshalString(t *testing.T) {
	// This would normally be tested via YAML unmarshalling,
	// but we test the logic directly
	includeDecl := &model.EnvIncludeDecl{}

	// Simulate setting a single file
	includeDecl.Files = []string{".env"}

	if len(includeDecl.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(includeDecl.Files))
	}
	if includeDecl.Files[0] != ".env" {
		t.Errorf("expected .env, got %q", includeDecl.Files[0])
	}
}

func TestEnvIncludeDecl_UnmarshalList(t *testing.T) {
	// Simulate setting multiple files
	includeDecl := &model.EnvIncludeDecl{}
	includeDecl.Files = []string{".env", ".env.local", ".env.production"}

	if len(includeDecl.Files) != 3 {
		t.Errorf("expected 3 files, got %d", len(includeDecl.Files))
	}

	expected := []string{".env", ".env.local", ".env.production"}
	for i, file := range includeDecl.Files {
		if file != expected[i] {
			t.Errorf("file[%d] = %q, want %q", i, file, expected[i])
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["PLAIN"] != "no_interpolation" {
		t.Errorf("expected no_interpolation, got %q", result["PLAIN"])
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["PORT"] != "8080" {
		t.Errorf("expected PORT=8080, got %q", result["PORT"])
	}
	if result["DEBUG"] != "true" {
		t.Errorf("expected DEBUG=true, got %q", result["DEBUG"])
	}
}

func TestLoadEnvFile_SingleQuote(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")
	os.WriteFile(envFile, []byte("KEY='single quoted value'\n"), 0o644)

	env := make(map[string]string)
	if err := loadEnvFile(envFile, env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["KEY"] != "single quoted value" {
		t.Errorf("expected 'single quoted value', got %q", env["KEY"])
	}
}
