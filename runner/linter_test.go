package runner

import (
	"testing"

	"github.com/titpetric/atkins-ci/model"
)

// TestLinter_ValidatesDependencies checks that the linter catches missing dependencies
func TestLinter_ValidateMissingDependency(t *testing.T) {
	pipeline := &model.Pipeline{
		Name: "test",
		Jobs: map[string]*model.Job{
			"job1": {
				DependsOn: "nonexistent",
			},
		},
	}

	linter := NewLinter(pipeline)
	errors := linter.Lint()

	if len(errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(errors))
	}

	if errors[0].Job != "job1" {
		t.Errorf("error job = %q, want 'job1'", errors[0].Job)
	}

	if errors[0].Issue != "missing dependency" {
		t.Errorf("error issue = %q, want 'missing dependency'", errors[0].Issue)
	}
}

// TestLinter_ValidDependencies checks that valid dependencies pass
func TestLinter_ValidDependencies(t *testing.T) {
	pipeline := &model.Pipeline{
		Name: "test",
		Jobs: map[string]*model.Job{
			"fmt": {},
			"test": {
				DependsOn: "fmt",
			},
		},
	}

	linter := NewLinter(pipeline)
	errors := linter.Lint()

	if len(errors) != 0 {
		t.Errorf("got %d errors, want 0", len(errors))
	}
}

// TestGetDependencies converts various formats to string slices
func TestGetDependencies(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{
			name:  "nil",
			input: nil,
			want:  []string{},
		},
		{
			name:  "single string",
			input: "job1",
			want:  []string{"job1"},
		},
		{
			name:  "string slice",
			input: []string{"job1", "job2"},
			want:  []string{"job1", "job2"},
		},
		{
			name:  "interface slice",
			input: []interface{}{"job1", "job2"},
			want:  []string{"job1", "job2"},
		},
		{
			name:  "interface slice with non-strings",
			input: []interface{}{"job1", 42, "job2"},
			want:  []string{"job1", "job2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDependencies(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("got %d deps, want %d", len(got), len(tt.want))
				return
			}
			for i, dep := range got {
				if dep != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, dep, tt.want[i])
				}
			}
		})
	}
}

// TestResolveJobDependencies returns jobs in correct order
func TestResolveJobDependencies_Order(t *testing.T) {
	jobs := map[string]*model.Job{
		"fmt":        {},
		"test":       {DependsOn: "fmt"},
		"test:build": {DependsOn: "fmt"},
		"test:run":   {DependsOn: []interface{}{"test:build", "test"}},
	}

	resolved, err := ResolveJobDependencies(jobs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// fmt should come first
	if resolved[0] != "fmt" {
		t.Errorf("first job = %q, want 'fmt'", resolved[0])
	}

	// test:run should come last (depends on others)
	if resolved[len(resolved)-1] != "test:run" {
		t.Errorf("last job = %q, want 'test:run'", resolved[len(resolved)-1])
	}
}

// TestResolveJobDependencies_SpecificJob resolves dependency chain for a specific job
func TestResolveJobDependencies_SpecificJob(t *testing.T) {
	jobs := map[string]*model.Job{
		"fmt":        {},
		"test":       {DependsOn: "fmt"},
		"test:build": {DependsOn: "fmt"},
		"test:run":   {DependsOn: "test:build"},
	}

	resolved, err := ResolveJobDependencies(jobs, "test:run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only include jobs needed for test:run
	expectedJobs := map[string]bool{
		"fmt":        true,
		"test:build": true,
		"test:run":   true,
	}

	if len(resolved) != len(expectedJobs) {
		t.Errorf("got %d jobs, want %d", len(resolved), len(expectedJobs))
	}

	for _, job := range resolved {
		if !expectedJobs[job] {
			t.Errorf("unexpected job in resolved: %q", job)
		}
	}

	// test:run should be last
	if resolved[len(resolved)-1] != "test:run" {
		t.Errorf("last job = %q, want 'test:run'", resolved[len(resolved)-1])
	}
}

// TestResolveJobDependencies_MissingJob returns error for nonexistent job
func TestResolveJobDependencies_MissingJob(t *testing.T) {
	jobs := map[string]*model.Job{
		"fmt": {},
	}

	_, err := ResolveJobDependencies(jobs, "nonexistent")
	if err == nil {
		t.Errorf("expected error for missing job, got nil")
	}
}
