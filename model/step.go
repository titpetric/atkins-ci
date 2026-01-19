package model

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Step represents a step within a job.
type Step struct {
	*Decl

	Name      string                 `yaml:"name,omitempty"`
	Desc      string                 `yaml:"desc,omitempty"`
	Run       string                 `yaml:"run,omitempty"`
	Cmd       string                 `yaml:"cmd,omitempty"`
	Cmds      []string               `yaml:"cmds,omitempty"`
	Task      string                 `yaml:"task,omitempty"` // Task/job name to invoke
	If        string                 `yaml:"if,omitempty"`
	For       string                 `yaml:"for,omitempty"`
	Uses      string                 `yaml:"uses,omitempty"`
	With      map[string]interface{} `yaml:"with,omitempty"`
	Detach    bool                   `yaml:"detach,omitempty"`
	Deferred  bool                   `yaml:"deferred,omitempty"`
	Verbose   bool                   `yaml:"verbose,omitempty"`
	Summarize bool                   `yaml:"summarize,omitempty"`
	Passthru  bool                   `yaml:"passthru,omitempty"` // If true, output is printed with tree indentation
	TTY       bool                   `yaml:"tty,omitempty"`      // If true, allocate a PTY for the command (enables color output)
}

// DeferredStep represents a deferred step wrapper.
type DeferredStep struct {
	Defer *Step `yaml:"defer,omitempty"`
}

// String returns a string representation of the step.
func (s *Step) String() string {
	switch {
	case s.Task != "":
		return "task: " + s.Task
	case s.Run != "":
		return "run: " + s.Run
	case s.Cmd != "":
		return "cmd: " + s.Cmd
	case len(s.Cmds) > 0:
		return fmt.Sprintf("cmds: [%s,...] (%d)", s.Cmds[0], len(s.Cmds))
	}
	return s.Name
}

// IsDeferred returns true if deferred is filled.
func (s *Step) IsDeferred() bool {
	return s.Deferred
}

// UnmarshalYAML implements custom unmarshalling for Step to support various formats and handle Decl.
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		// Simple string step - treat as a Run command
		s.Run = strings.TrimSpace(node.Value)
		s.Name = strings.TrimSpace(node.Value)
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

			return nil
		}

		// Ensure Decl is initialized and vars/include are properly decoded
		if err := ensureDeclInitialized(&s.Decl, node); err != nil {
			return err
		}

		// Trim spaces from Run, Cmd, and Cmds after decoding
		s.Run = strings.TrimSpace(s.Run)
		s.Cmd = strings.TrimSpace(s.Cmd)
		for i, cmd := range s.Cmds {
			s.Cmds[i] = strings.TrimSpace(cmd)
		}

		return nil
	}

	return fmt.Errorf("invalid step format: expected string or object, got %v", node.Kind)
}
