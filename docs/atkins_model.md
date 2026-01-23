# Package ./model

```go
import (
	"github.com/titpetric/atkins/model"
}
```

## Types

```go
// Decl represents a common variables signature {vars, env, include}.
//
// It's a base type for pipelines, jobs/tasks and steps/cmds.
type Decl struct {
	Vars	map[string]any	`yaml:"vars,omitempty"`
	Include	*IncludeDecl	`yaml:"include,omitempty"`
	Env	*EnvDecl	`yaml:"env,omitempty"`
}
```

```go
// DeferredStep represents a deferred step wrapper.
type DeferredStep struct {
	Defer *Step `yaml:"defer,omitempty"`
}
```

```go
// Dependencies represents job dependencies.
type Dependencies []string
```

```go
// EnvDecl represents an environment variable declaration that can contain
// both manually-set variables and includes from external files.
type EnvDecl Decl
```

```go
// IncludeDecl represents file includes that can be either a single string or a list of strings.
type IncludeDecl struct {
	Files []string
}
```

```go
// Job represents a job/task in the pipeline.
type Job struct {
	*Decl

	Desc		string		`yaml:"desc,omitempty"`
	RunsOn		string		`yaml:"runs_on,omitempty"`
	Container	string		`yaml:"container,omitempty"`
	If		string		`yaml:"if,omitempty"`
	Cmd		string		`yaml:"cmd,omitempty"`
	Cmds		[]*Step		`yaml:"cmds,omitempty"`
	Run		string		`yaml:"run,omitempty"`
	Steps		[]*Step		`yaml:"steps,omitempty"`
	Detach		bool		`yaml:"detach,omitempty"`
	Show		*bool		`yaml:"show,omitempty"`	// Show in display (true=show, false=hide, nil=show if root level/ invoked)
	DependsOn	Dependencies	`yaml:"depends_on,omitempty"`
	Requires	[]string	`yaml:"requires,omitempty"`	// Variables required when invoked in a loop
	Timeout		string		`yaml:"timeout,omitempty"`	// e.g., "10m", "300s"
	Summarize	bool		`yaml:"summarize,omitempty"`
	Passthru	bool		`yaml:"passthru,omitempty"`	// If true, output is printed with tree indentation
	TTY		bool		`yaml:"tty,omitempty"`		// If true, allocate a PTY for all steps (enables color output)

	Name	string	`yaml:"-"`
	Nested	bool	`yaml:"-"`
}
```

```go
// Label represents a display label for a step or command.
type Label struct {
	Text		string			// The display text (e.g., "docker compose up" or "run: goimports -w .")
	Type		string			// The type of operation: "task", "run", "cmd", "cmds"
	ShowPrefix	bool			// Whether to display the type prefix (e.g., "run:")
	Status		string			// Optional status indicator (e.g., "●", "✓", "✗")
	Color		func(string) string	// Optional color function to apply to the label
}
```

```go
// Pipeline represents the root structure of an atkins.yml file.
type Pipeline struct {
	*Decl

	Name	string		`yaml:"name,omitempty"`
	Jobs	map[string]*Job	`yaml:"jobs,omitempty"`
	Tasks	map[string]*Job	`yaml:"tasks,omitempty"`
}
```

```go
// Step represents a step within a job.
type Step struct {
	*Decl

	Name		string			`yaml:"name,omitempty"`
	Desc		string			`yaml:"desc,omitempty"`
	Run		string			`yaml:"run,omitempty"`
	Cmd		string			`yaml:"cmd,omitempty"`
	Cmds		[]string		`yaml:"cmds,omitempty"`
	Task		string			`yaml:"task,omitempty"`	// Task/job name to invoke
	If		string			`yaml:"if,omitempty"`
	For		string			`yaml:"for,omitempty"`
	Uses		string			`yaml:"uses,omitempty"`
	With		map[string]interface{}	`yaml:"with,omitempty"`
	Detach		bool			`yaml:"detach,omitempty"`
	Deferred	bool			`yaml:"deferred,omitempty"`
	Verbose		bool			`yaml:"verbose,omitempty"`
	Summarize	bool			`yaml:"summarize,omitempty"`
	Passthru	bool			`yaml:"passthru,omitempty"`	// If true, output is printed with tree indentation
	TTY		bool			`yaml:"tty,omitempty"`		// If true, allocate a PTY for the command (enables color output)
	HidePrefix	bool			`yaml:"-"`			// If true, don't show "run:" prefix in display
}
```

## Function symbols

- `func NewLabel (text,labelType string) *Label`
- `func (*Dependencies) UnmarshalYAML (node *yaml.Node) error`
- `func (*IncludeDecl) UnmarshalYAML (node *yaml.Node) error`
- `func (*Job) Children () []*Step`
- `func (*Job) IsRootLevel () bool`
- `func (*Job) ShouldShow () bool`
- `func (*Job) UnmarshalYAML (node *yaml.Node) error`
- `func (*Label) ForDisplay () string`
- `func (*Label) String () string`
- `func (*Label) WithColor (colorFn func(string) string) *Label`
- `func (*Label) WithPrefix (show bool) *Label`
- `func (*Label) WithStatus (status string) *Label`
- `func (*Pipeline) UnmarshalYAML (node *yaml.Node) error`
- `func (*Step) Commands () []string`
- `func (*Step) DisplayLabel () string`
- `func (*Step) IsDeferred () bool`
- `func (*Step) Label (showPrefix bool) *Label`
- `func (*Step) String () string`
- `func (*Step) UnmarshalYAML (node *yaml.Node) error`

### NewLabel

NewLabel creates a new label with the given text and type.
Use builder methods like WithPrefix(), WithStatus(), and WithColor() to customize.

```go
func NewLabel (text,labelType string) *Label
```

### UnmarshalYAML

UnmarshalYAML implements custom unmarshalling for `depends_on`,
taking a string value, or a slice of strings.

```go
func (*Dependencies) UnmarshalYAML (node *yaml.Node) error
```

### UnmarshalYAML

UnmarshalYAML implements custom unmarshalling for IncludeDecl to support string or []string.

```go
func (*IncludeDecl) UnmarshalYAML (node *yaml.Node) error
```

### Children

Children returns job steps for execution.

```go
func (*Job) Children () []*Step
```

### IsRootLevel

IsRootLevel returns true if the job is a root-level job (no ':' in name).

```go
func (*Job) IsRootLevel () bool
```

### ShouldShow

ShouldShow returns true if the job should be displayed in the tree output.
Root jobs are shown by default. Nested jobs are hidden unless Show is true.

```go
func (*Job) ShouldShow () bool
```

### UnmarshalYAML

UnmarshalYAML implements custom unmarshalling for Job to trim whitespace and handle Decl.
It also supports parsing a job from a simple string (e.g., "up: docker compose up").

```go
func (*Job) UnmarshalYAML (node *yaml.Node) error
```

### ForDisplay

ForDisplay returns the formatted label with color and status for rendering.
This includes ANSI color codes and status indicators.

```go
func (*Label) ForDisplay () string
```

### String

String returns the formatted label as a clean string (no ANSI codes or status).
Use this for node names and comparisons.

```go
func (*Label) String () string
```

### WithColor

WithColor sets a color function to apply to the label.

```go
func (*Label) WithColor (colorFn func(string) string) *Label
```

### WithPrefix

WithPrefix sets whether to show the type prefix in output.

```go
func (*Label) WithPrefix (show bool) *Label
```

### WithStatus

WithStatus sets the status indicator to display.

```go
func (*Label) WithStatus (status string) *Label
```

### UnmarshalYAML

UnmarshalYAML implements custom unmarshalling for Pipeline to handle Decl.

```go
func (*Pipeline) UnmarshalYAML (node *yaml.Node) error
```

### Commands

Commands returns all executable commands from this step as a slice.
For steps with a single command (Run/Cmd), returns a slice with one element.
For steps with multiple commands (Cmds), returns the full slice.
Returns an empty slice for Task steps.

```go
func (*Step) Commands () []string
```

### DisplayLabel

DisplayLabel returns a display label for the step, always showing prefixes.
This is used during execution to clearly show what type of operation is being run.

```go
func (*Step) DisplayLabel () string
```

### IsDeferred

IsDeferred returns true if deferred is filled.

```go
func (*Step) IsDeferred () bool
```

### Label

Label returns a structured label for the step with information about how to display it.
showPrefix controls whether to include the type prefix (e.g., "run:", "task:").
HidePrefix overrides this to never show the prefix (for simple shorthand tasks).

```go
func (*Step) Label (showPrefix bool) *Label
```

### String

String returns a string representation of the step.

```go
func (*Step) String () string
```

### UnmarshalYAML

UnmarshalYAML implements custom unmarshalling for Step to support various formats and handle Decl.

```go
func (*Step) UnmarshalYAML (node *yaml.Node) error
```


