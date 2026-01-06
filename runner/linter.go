package runner

import (
	"fmt"
	"maps"
	"slices"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/treeview"
)

// LintError represents a linting error.
type LintError struct {
	Job    string
	Issue  string
	Detail string
}

// Linter validates a pipeline for correctness.
type Linter struct {
	pipeline *model.Pipeline
	errors   []LintError
}

// NewLinter creates a new linter.
func NewLinter(pipeline *model.Pipeline) *Linter {
	return &Linter{
		pipeline: pipeline,
		errors:   make([]LintError, 0),
	}
}

// Lint validates the pipeline and returns any errors.
func (l *Linter) Lint() []LintError {
	l.validateDependencies()
	l.validateTaskInvocations()
	return l.errors
}

// validateDependencies checks that all depends_on references exist
func (l *Linter) validateDependencies() {
	jobs := l.pipeline.Jobs
	if len(jobs) == 0 {
		jobs = l.pipeline.Tasks
	}

	for jobName, job := range jobs {
		if job == nil {
			continue
		}

		deps := GetDependencies(job.DependsOn)
		for _, dep := range deps {
			if _, exists := jobs[dep]; !exists {
				l.errors = append(l.errors, LintError{
					Job:    jobName,
					Issue:  "missing dependency",
					Detail: fmt.Sprintf("job '%s' depends_on '%s', but job '%s' not found", jobName, dep, dep),
				})
			}
		}
	}
}

// validateTaskInvocations checks that referenced tasks exist
func (l *Linter) validateTaskInvocations() {
	jobs := l.pipeline.Jobs
	if len(jobs) == 0 {
		jobs = l.pipeline.Tasks
	}

	for jobName, job := range jobs {
		if job == nil {
			continue
		}

		// Check each step for task references
		for _, step := range job.Steps {
			if step != nil && step.Task != "" {
				if _, exists := jobs[step.Task]; !exists {
					l.errors = append(l.errors, LintError{
						Job:    jobName,
						Issue:  "missing task reference",
						Detail: fmt.Sprintf("step references task '%s', but task not found", step.Task),
					})
				}
			}
		}
	}
}

// GetDependencies converts depends_on field (string or []string) to a slice of job names.
func GetDependencies(dependsOn interface{}) []string {
	if dependsOn == nil {
		return []string{}
	}

	switch v := dependsOn.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	case model.Dependencies:
		return []string(v)
	default:
		return []string{}
	}
}

// ResolveJobDependencies returns jobs in dependency order.
// Returns the jobs to run and any resolution errors.
func ResolveJobDependencies(jobs map[string]*model.Job, startingJob string) ([]string, error) {
	if len(jobs) == 0 {
		return []string{}, nil
	}

	// If a specific job is requested, resolve its dependency chain
	if startingJob != "" {
		if _, exists := jobs[startingJob]; !exists {
			return nil, fmt.Errorf("job '%s' not found", startingJob)
		}
		return resolveDependencyChain(jobs, startingJob)
	}

	// Otherwise, resolve root jobs (those without ':' in name)
	// Mark nested jobs so they won't be executed directly
	for name, job := range jobs {
		if job.Name == "" {
			job.Name = name
		}
		if !job.IsRootLevel() {
			job.Nested = true
		}
	}

	// If 'default' job exists, start with that
	if _, hasDefault := jobs["default"]; hasDefault && len(jobs) > 0 {
		return resolveDependencyChain(jobs, "default")
	}

	return resolveJobs(jobs)
}

// resolveDependencyChain returns a job and all its dependencies in execution order
func resolveDependencyChain(jobs map[string]*model.Job, jobName string) ([]string, error) {
	// Set Name field on all jobs for IsRootLevel() check
	for name, job := range jobs {
		if job.Name == "" {
			job.Name = name
		}
	}

	resolved := make([]string, 0)
	visited := make(map[string]bool)
	var visit func(string) error

	visit = func(name string) error {
		if visited[name] {
			return nil // Already visited
		}

		job, exists := jobs[name]
		if !exists {
			return fmt.Errorf("job '%s' not found", name)
		}

		visited[name] = true

		// Visit dependencies first
		deps := GetDependencies(job.DependsOn)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}

		resolved = append(resolved, name)
		return nil
	}

	if err := visit(jobName); err != nil {
		return nil, err
	}

	return resolved, nil
}

// ValidateJobRequirements checks that all required variables are present in the context.
// Returns an error with a clear message listing missing variables.
func ValidateJobRequirements(job *model.Job, ctx *ExecutionContext) error {
	if len(job.Requires) == 0 {
		return nil // No requirements to validate
	}

	var missing []string
	for _, varName := range job.Requires {
		if _, exists := ctx.Variables[varName]; !exists {
			missing = append(missing, varName)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("job '%s' requires variables %v but missing: %v", job.Name, job.Requires, missing)
	}

	return nil
}

// resolveJobs returns all jobs in dependency order (topological sort)
// When called without a specific job, only root jobs are traversed as starting points,
// but their nested dependencies are included in the result.
func resolveJobs(jobs map[string]*model.Job) ([]string, error) {
	resolved := make([]string, 0)
	visited := make(map[string]bool)
	var visit func(string) error

	visit = func(name string) error {
		if visited[name] {
			return nil
		}

		job, exists := jobs[name]
		if !exists {
			return fmt.Errorf("job '%s' not found", name)
		}

		visited[name] = true

		// Visit dependencies first
		deps := GetDependencies(job.DependsOn)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}

		resolved = append(resolved, name)
		return nil
	}

	// Only start traversal from root jobs
	// This ensures nested jobs are only included if they're dependencies of root jobs
	// Sort by depth for consistent ordering with static display
	names := treeview.SortJobsByDepth(slices.Sorted(maps.Keys(jobs)))
	for _, jobName := range names {
		job := jobs[jobName]
		if job.Name == "" {
			job.Name = jobName
		}
		if !job.IsRootLevel() {
			continue // Skip nested jobs as starting points
		}
		if err := visit(jobName); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}
