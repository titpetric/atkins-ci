package model

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Step represents a step within a job.
type Step struct {
	*Decl

	Name       string                 `yaml:"name,omitempty"`
	Desc       string                 `yaml:"desc,omitempty"`
	Run        string                 `yaml:"run,omitempty"`
	Cmd        string                 `yaml:"cmd,omitempty"`
	Cmds       []string               `yaml:"cmds,omitempty"`
	Task       string                 `yaml:"task,omitempty"` // Task/job name to invoke
	If         string                 `yaml:"if,omitempty"`
	For        string                 `yaml:"for,omitempty"`
	Uses       string                 `yaml:"uses,omitempty"`
	With       map[string]interface{} `yaml:"with,omitempty"`
	Detach     bool                   `yaml:"detach,omitempty"`
	Deferred   bool                   `yaml:"deferred,omitempty"`
	Verbose    bool                   `yaml:"verbose,omitempty"`
	Summarize  bool                   `yaml:"summarize,omitempty"`
	Passthru   bool                   `yaml:"passthru,omitempty"` // If true, output is printed with tree indentation
	TTY        bool                   `yaml:"tty,omitempty"`      // If true, allocate a PTY for the command (enables color output)
	HidePrefix bool                   `yaml:"-"`                  // If true, don't show "run:" prefix in display
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
		// If Run contains newlines, display as <script> instead of full command
		if strings.Contains(s.Run, "\n") {
			return "run: <script>"
		}
		return "run: " + s.Run
	case s.Cmd != "":
		return "cmd: " + s.Cmd
	case len(s.Cmds) > 0:
		return fmt.Sprintf("cmds: <%d commands>", len(s.Cmds))
	}
	return s.Name
}

// DisplayLabel returns a display label for the step, always showing prefixes.
// This is used during execution to clearly show what type of operation is being run.
func (s *Step) DisplayLabel() string {
	switch {
	case s.Task != "":
		return "task: " + s.Task
	case s.Run != "":
		// If Run contains newlines, display as <script> instead of full command
		if strings.Contains(s.Run, "\n") {
			return "run: <script>"
		}
		return "run: " + s.Run
	case s.Cmd != "":
		return "cmd: " + s.Cmd
	case len(s.Cmds) > 0:
		return fmt.Sprintf("cmds: <%d commands>", len(s.Cmds))
	}
	return s.Name
}

// Label returns a structured label for the step with information about how to display it.
// showPrefix controls whether to include the type prefix (e.g., "run:", "task:").
// HidePrefix overrides this to never show the prefix (for simple shorthand tasks).
func (s *Step) Label(showPrefix bool) *Label {
	switch {
	case s.Task != "":
		return &Label{
			Text:       s.Task,
			Type:       "task",
			ShowPrefix: showPrefix && !s.HidePrefix,
		}
	case s.Run != "":
		text := s.Run
		if strings.Contains(text, "\n") {
			text = "<script>"
		}
		return &Label{
			Text:       text,
			Type:       "run",
			ShowPrefix: showPrefix && !s.HidePrefix,
		}
	case s.Cmd != "":
		return &Label{
			Text:       s.Cmd,
			Type:       "cmd",
			ShowPrefix: showPrefix && !s.HidePrefix,
		}
	case len(s.Cmds) > 0:
		return &Label{
			Text:       fmt.Sprintf("<%d commands>", len(s.Cmds)),
			Type:       "cmds",
			ShowPrefix: showPrefix && !s.HidePrefix,
		}
	}
	return &Label{
		Text:       s.Name,
		Type:       "default",
		ShowPrefix: false,
	}
}

// Commands returns all executable commands from this step as a slice.
// For steps with a single command (Run/Cmd), returns a slice with one element.
// For steps with multiple commands (Cmds), returns the full slice.
// Returns an empty slice for Task steps.
func (s *Step) Commands() []string {
	if len(s.Cmds) > 0 {
		return s.Cmds
	}
	if s.Run != "" {
		return []string{s.Run}
	}
	if s.Cmd != "" {
		return []string{s.Cmd}
	}
	return []string{}
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
