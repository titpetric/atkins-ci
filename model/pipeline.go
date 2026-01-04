package model

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Pipeline represents the root structure of an atkins.yml file
type Pipeline struct {
	Name  string          `yaml:"name,omitempty"`
	Jobs  map[string]*Job `yaml:"jobs,omitempty"`
	Tasks map[string]*Job `yaml:"tasks,omitempty"`
}

// Job represents a job/task in the pipeline
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

// Step represents a step within a job
type Step struct {
	Name     string                 `yaml:"name,omitempty"`
	Desc     string                 `yaml:"desc,omitempty"`
	Run      string                 `yaml:"run,omitempty"`
	Cmd      string                 `yaml:"cmd,omitempty"`
	Cmds     []string               `yaml:"cmds,omitempty"`
	If       string                 `yaml:"if,omitempty"`
	For      string                 `yaml:"for,omitempty"`
	Env      map[string]string      `yaml:"env,omitempty"`
	Uses     string                 `yaml:"uses,omitempty"`
	With     map[string]interface{} `yaml:"with,omitempty"`
	Detach   bool                   `yaml:"detach,omitempty"`
	Defer    string                 `yaml:"defer,omitempty"`
	Deferred bool                   `yaml:"deferred,omitempty"`
	Verbose  bool                   `yaml:"verbose,omitempty"`
}

type Dependencies []string

// UnmarshalYAML implements custom unmarshalling for `depends_on`,
// taking a string value, or a slice of strings.
func (s *Dependencies) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = Dependencies([]string{node.Value})
		return nil
	}

	var deps []string
	if err := node.Decode(&deps); err != nil {
		return err
	}
	*s = Dependencies(deps)
	return nil
}

// UnmarshalYAML implements custom unmarshalling for Step to support various formats
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		// Simple string step - treat as a Run command
		s.Run = node.Value
		s.Name = node.Value
		return nil
	}

	if node.Kind == yaml.MappingNode {
		// Object step - use default unmarshalling
		type rawStep Step
		if err := node.Decode((*rawStep)(s)); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("invalid step format: expected string or object, got %v", node.Kind)
}

// Service represents a service (e.g., Docker container) used in a job
type Service struct {
	Image    string            `yaml:"image,omitempty"`
	Pull     string            `yaml:"pull,omitempty"`
	Options  string            `yaml:"options,omitempty"`
	Ports    []string          `yaml:"ports,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Networks []string          `yaml:"networks,omitempty"`
}
