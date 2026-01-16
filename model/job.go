package model

import (
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Job represents a job/task in the pipeline.
type Job struct {
	*Decl

	Desc      string       `yaml:"desc,omitempty"`
	RunsOn    string       `yaml:"runs_on,omitempty"`
	Container string       `yaml:"container,omitempty"`
	If        string       `yaml:"if,omitempty"`
	Cmd       string       `yaml:"cmd,omitempty"`
	Cmds      []string     `yaml:"cmds,omitempty"`
	Run       string       `yaml:"run,omitempty"`
	Steps     []*Step      `yaml:"steps,omitempty"`
	Detach    bool         `yaml:"detach,omitempty"`
	Show      *bool        `yaml:"show,omitempty"` // Show in display (true=show, false=hide, nil=show if root level/ invoked)
	DependsOn Dependencies `yaml:"depends_on,omitempty"`
	Requires  []string     `yaml:"requires,omitempty"` // Variables required when invoked in a loop
	Timeout   string       `yaml:"timeout,omitempty"`  // e.g., "10m", "300s"
	Summarize bool         `yaml:"summarize,omitempty"`
	Passthru  bool         `yaml:"passthru,omitempty"` // If true, output is printed with tree indentation

	Name   string `yaml:"-"`
	Nested bool   `yaml:"-"`
}

// IsRootLevel returns true if the job is a root-level job (no ':' in name).
func (j *Job) IsRootLevel() bool {
	// A job is root-level if it doesn't contain ':' in its name
	for _, ch := range j.Name {
		if ch == ':' {
			return false
		}
	}
	return true
}

// ShouldShow returns true if the job should be displayed in the tree output.
// Root jobs are shown by default. Nested jobs are hidden unless Show is true.
func (j *Job) ShouldShow() bool {
	// Always show if explicitly marked with show: true
	if j.Show != nil {
		return *j.Show
	}
	// Show root jobs by default, hide nested jobs
	return j.IsRootLevel()
}

// UnmarshalYAML implements custom unmarshalling for Job to trim whitespace and handle Decl.
// It also supports parsing a job from a simple string (e.g., "up: docker compose up").
func (j *Job) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		// Simple string job - treat as a job with a single step
		cmd := strings.TrimSpace(node.Value)
		j.Steps = []*Step{{Run: cmd, Name: cmd}}
		j.Passthru = true
		return nil
	}

	type rawJob Job
	if err := node.Decode((*rawJob)(j)); err != nil {
		return err
	}

	// Ensure Decl is initialized and vars/include are properly decoded
	if err := ensureDeclInitialized(&j.Decl, node); err != nil {
		return err
	}

	// Trim spaces from Run, Cmd, and Cmds after decoding
	j.Run = strings.TrimSpace(j.Run)
	j.Cmd = strings.TrimSpace(j.Cmd)
	for i, cmd := range j.Cmds {
		j.Cmds[i] = strings.TrimSpace(cmd)
	}

	return nil
}
