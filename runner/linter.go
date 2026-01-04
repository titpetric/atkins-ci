package runner

import (
	"fmt"
	"strings"

	"github.com/titpetric/atkins-ci/model"
)

// LintError represents a linting error
type LintError struct {
	Job    string
	Issue  string
	Detail string
}

// Linter validates a pipeline for correctness
type Linter struct {
	pipeline *model.Pipeline
	errors   []LintError
}

// NewLinter creates a new linter
func NewLinter(pipeline *model.Pipeline) *Linter {
	return &Linter{
		pipeline: pipeline,
		errors:   make([]LintError, 0),
	}
}

// Lint validates the pipeline and returns any errors
func (l *Linter) Lint() []LintError {
	l.validateDependencies()
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

// GetDependencies converts depends_on field (string or []string) to a slice of job names
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
	default:
		return []string{}
	}
}

// ResolveJobDependencies returns jobs in dependency order
// Returns the jobs to run and any resolution errors
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
	rootJobs := make(map[string]*model.Job)
	for name, job := range jobs {
		if !strings.Contains(name, ":") {
			rootJobs[name] = job
		}
	}

	// If 'default' job exists, start with that
	if _, hasDefault := rootJobs["default"]; hasDefault && len(rootJobs) > 0 {
		return resolveDependencyChain(rootJobs, "default")
	}

	// Otherwise resolve all root jobs
	if len(rootJobs) > 0 {
		return resolveAllJobs(rootJobs)
	}

	// Fallback: if no root jobs, resolve all jobs
	return resolveAllJobs(jobs)
}

// resolveDependencyChain returns a job and all its dependencies in execution order
func resolveDependencyChain(jobs map[string]*model.Job, jobName string) ([]string, error) {
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

// resolveAllJobs returns all jobs in dependency order (topological sort)
func resolveAllJobs(jobs map[string]*model.Job) ([]string, error) {
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

	// Visit all jobs
	for jobName := range jobs {
		if err := visit(jobName); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}
