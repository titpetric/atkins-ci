package model

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Pipeline represents the root structure of an atkins.yml file.
type Pipeline struct {
	Name  string          `yaml:"name,omitempty"`
	Jobs  map[string]*Job `yaml:"jobs,omitempty"`
	Tasks map[string]*Job `yaml:"tasks,omitempty"`
}

// Job represents a job/task in the pipeline.
type Job struct {
	Desc      string                 `yaml:"desc,omitempty"`
	RunsOn    string                 `yaml:"runs_on,omitempty"`
	Container string                 `yaml:"container,omitempty"`
	If        string                 `yaml:"if,omitempty"`
	Cmd       string                 `yaml:"cmd,omitempty"`
	Cmds      []string               `yaml:"cmds,omitempty"`
	Run       string                 `yaml:"run,omitempty"`
	Steps     []*Step                `yaml:"steps,omitempty"`
	Services  map[string]*Service    `yaml:"services,omitempty"`
	Vars      map[string]interface{} `yaml:"vars,omitempty"`
	Env       map[string]string      `yaml:"env,omitempty"`
	Detach    bool                   `yaml:"detach,omitempty"`
	DependsOn Dependencies           `yaml:"depends_on,omitempty"`
	Timeout   string                 `yaml:"timeout,omitempty"` // e.g., "10m", "300s"

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

// Step represents a step within a job.
type Step struct {
	Name     string                 `yaml:"name,omitempty"`
	Desc     string                 `yaml:"desc,omitempty"`
	Run      string                 `yaml:"run,omitempty"`
	Cmd      string                 `yaml:"cmd,omitempty"`
	Cmds     []string               `yaml:"cmds,omitempty"`
	Task     string                 `yaml:"task,omitempty"` // Task/job name to invoke
	If       string                 `yaml:"if,omitempty"`
	For      string                 `yaml:"for,omitempty"`
	Env      map[string]string      `yaml:"env,omitempty"`
	Uses     string                 `yaml:"uses,omitempty"`
	With     map[string]interface{} `yaml:"with,omitempty"`
	Detach   bool                   `yaml:"detach,omitempty"`
	Deferred bool                   `yaml:"deferred,omitempty"`
	Verbose  bool                   `yaml:"verbose,omitempty"`
}

type DeferredStep struct {
	Defer *Step `yaml:"defer,omitempty"`
}

// IsDeferred returns true if deferred is filled.
func (s *Step) IsDeferred() bool {
	return s.Deferred
}

func (s *Step) String() string {
	switch {
	case s.Task != "":
		return s.Task
	case s.Run != "":
		return s.Run
	case s.Cmd != "":
		return s.Cmd
	case len(s.Cmds) > 0:
		// For cmds array, just show the first one as a placeholder
		return s.Cmds[0]
	}
	return s.Name
}

// UnmarshalYAML implements custom unmarshalling for Step to support various formats.
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		// Simple string step - treat as a Run command
		s.Run = node.Value
		s.Name = node.Value
		return nil
	}

	if node.Kind == yaml.MappingNode {
		type rawStep Step
		if err := node.Decode((*rawStep)(s)); err != nil {
			return err
		}

		var ds DeferredStep
		if err := node.Decode(&ds); err != nil {
			return err
		}

		if ds.Defer != nil {
			if s.Run != "" {
				return fmt.Errorf("Error processing step: step has run %s, and defer %s. Should use {defer} or {run, deferred=true}.", s.Run, ds.Defer.Run)
			}

			*s = *ds.Defer
			s.Deferred = true

			fmt.Printf("step: %#v\n", s.String())

			return nil
		}

		return nil
	}

	return fmt.Errorf("invalid step format: expected string or object, got %v", node.Kind)
}

// Service represents a service (e.g., Docker container) used in a job.
type Service struct {
	Image    string            `yaml:"image,omitempty"`
	Pull     string            `yaml:"pull,omitempty"`
	Options  string            `yaml:"options,omitempty"`
	Ports    []string          `yaml:"ports,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Networks []string          `yaml:"networks,omitempty"`
}
