package model

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Pipeline represents the root structure of an atkins.yml file
type Pipeline struct {
	Name  string          `yaml:"name"`
	Jobs  map[string]*Job `yaml:"jobs"`
	Tasks map[string]*Job `yaml:"tasks"`
}

// Job represents a job/task in the pipeline
type Job struct {
	Desc      string                 `yaml:"desc"`
	RunsOn    string                 `yaml:"runs_on"`
	Container string                 `yaml:"container"`
	If        string                 `yaml:"if"`
	Cmd       string                 `yaml:"cmd"`
	Cmds      []string               `yaml:"cmds"`
	Run       string                 `yaml:"run"`
	Steps     []Step                 `yaml:"steps"`
	Services  map[string]*Service    `yaml:"services"`
	Vars      map[string]interface{} `yaml:"vars"`
	Env       map[string]string      `yaml:"env"`
	Matrix    map[string]interface{} `yaml:"matrix"`
	Detach    bool                   `yaml:"detach"`
	DependsOn interface{}            `yaml:"depends_on"` // string or []string
	Timeout   string                 `yaml:"timeout"`    // e.g., "10m", "300s"
}

// Step represents a step within a job
type Step struct {
	Name   string                 `yaml:"name"`
	Desc   string                 `yaml:"desc"`
	Run    string                 `yaml:"run"`
	Cmd    string                 `yaml:"cmd"`
	Cmds   []string               `yaml:"cmds"`
	If     string                 `yaml:"if"`
	For    string                 `yaml:"for"`
	Env    map[string]string      `yaml:"env"`
	Uses   string                 `yaml:"uses"`
	With   map[string]interface{} `yaml:"with"`
	Detach   bool                   `yaml:"detach"`
	Defer    string                 `yaml:"defer"`
	Deferred bool                   `yaml:"deferred"`
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
	Image    string            `yaml:"image"`
	Pull     string            `yaml:"pull"`
	Options  string            `yaml:"options"`
	Ports    []string          `yaml:"ports"`
	Env      map[string]string `yaml:"env"`
	Networks []string          `yaml:"networks"`
}

// ExecutionContext holds runtime state during pipeline execution
type ExecutionContext struct {
	Variables   map[string]interface{}
	Env         map[string]string
	Results     map[string]interface{}
	QuietMode   int             // 0 = normal, 1 = quiet (no stdout), 2 = very quiet (no stdout/stderr)
	Pipeline    string          // Current pipeline name
	Job         string          // Current job name
	JobDesc     string          // Current job description
	Step        string          // Current step name
	Depth       int             // Nesting depth for indentation
	StepsCount  int             // Total number of steps executed
	StepsPassed int             // Number of steps that passed
	Tree        interface{}     // *ExecutionTree (avoid circular import)
	CurrentJob  interface{}     // *TreeNode for current job
	CurrentStep interface{}     // *TreeNode for current step
	Renderer    interface{}     // *TreeRenderer for in-place rendering
	Context     context.Context // Context for timeout and cancellation
}
