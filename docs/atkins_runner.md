# Package ./runner

```go
import (
	"github.com/titpetric/atkins/runner"
}
```

## Types

```go
// Exec runs shell commands.
type Exec struct {
	Env map[string]string	// Optional environment variables to pass to commands
}
```

```go
// ExecError represents an error from command execution.
type ExecError struct {
	Message		string
	Output		string
	LastExitCode	int
	Trace		string
}
```

```go
// ExecutionContext holds runtime state during pipeline Exec.
type ExecutionContext struct {
	Context	context.Context

	Env	map[string]string
	Results	map[string]any
	Verbose	bool

	Variables	map[string]any

	Pipeline	*model.Pipeline
	Job		*model.Job
	Step		*model.Step

	Depth		int	// Nesting depth for indentation
	StepsCount	int	// Total number of steps executed
	StepsPassed	int	// Number of steps that passed

	CurrentJob	*treeview.TreeNode
	CurrentStep	*treeview.Node

	Display		*treeview.Display
	Builder		*treeview.Builder
	JobNodes	map[string]*treeview.TreeNode	// Map of job names to their tree nodes
	EventLogger	*eventlog.Logger

	// Sequential step counter for this job (incremented for each step execution)
	StepSequence	int
	stepSeqMu	sync.Mutex
}
```

```go
// Executor runs pipeline jobs and steps.
type Executor struct {
	opts *Options
}
```

```go
// IterationContext holds the variables for a single iteration of a for loop.
type IterationContext struct {
	Variables map[string]any
}
```

```go
// LineCapturingWriter captures all output written to it.
type LineCapturingWriter struct {
	buffer	bytes.Buffer
	mu	sync.Mutex
}
```

```go
// LintError represents a linting error.
type LintError struct {
	Job	string
	Issue	string
	Detail	string
}
```

```go
// Linter validates a pipeline for correctness.
type Linter struct {
	pipeline	*model.Pipeline
	errors		[]LintError
}
```

```go
// Options provides configuration for the executor.
type Options struct {
	DefaultTimeout time.Duration
}
```

```go
// PipelineOptions contains options for running a pipeline.
type PipelineOptions struct {
	Job		string
	LogFile		string
	PipelineFile	string
	Debug		bool
	FinalOnly	bool
}
```

## Vars

```go
// ConfigNames are the default config file names to search for, in order of preference.
var ConfigNames = []string{".atkins.yml", ".atkins.yaml", "atkins.yml", "atkins.yaml"}
```

## Function symbols

- `func DefaultOptions () *Options`
- `func DiscoverConfig (startDir string) (string, error)`
- `func DiscoverConfigFromCwd () (string, error)`
- `func EvaluateIf (ctx *ExecutionContext) (bool, error)`
- `func ExpandFor (ctx *ExecutionContext, executeCommand func(string) (string, error)) ([]IterationContext, error)`
- `func GetDependencies (dependsOn any) []string`
- `func InterpolateCommand (cmd string, ctx *ExecutionContext) (string, error)`
- `func InterpolateMap (ctx *ExecutionContext, m map[string]any) error`
- `func InterpolateString (s string, ctx *ExecutionContext) (string, error)`
- `func IsEchoCommand (cmd string) bool`
- `func ListPipeline (pipeline *model.Pipeline) error`
- `func LoadPipeline (filePath string) ([]*model.Pipeline, error)`
- `func MergeVariables (decl *model.Decl, ctx *ExecutionContext) error`
- `func NewExec () *Exec`
- `func NewExecWithEnv (env map[string]string) *Exec`
- `func NewExecutor () *Executor`
- `func NewExecutorWithOptions (opts *Options) *Executor`
- `func NewLineCapturingWriter () *LineCapturingWriter`
- `func NewLinter (pipeline *model.Pipeline) *Linter`
- `func ProcessDecl (decl *model.Decl, ctx *ExecutionContext) (map[string]any, error)`
- `func ResolveJobDependencies (jobs map[string]*model.Job, startingJob string) ([]string, error)`
- `func RunPipeline (ctx context.Context, pipeline *model.Pipeline, opts PipelineOptions) error`
- `func Sanitize (in string) ([]string, error)`
- `func StripANSI (in string) string`
- `func ValidateJobRequirements (job *model.Job, ctx *ExecutionContext) error`
- `func VisualLength (s string) int`
- `func (*Exec) ExecuteCommand (cmdStr string) (string, error)`
- `func (*Exec) ExecuteCommandWithQuiet (cmdStr string, verbose bool) (string, error)`
- `func (*Exec) ExecuteCommandWithQuietAndCapture (cmdStr string, verbose bool) (string, error)`
- `func (*Exec) ExecuteCommandWithWriter (writer io.Writer, cmdStr string, usePTY bool) (string, error)`
- `func (*ExecutionContext) Copy () *ExecutionContext`
- `func (*ExecutionContext) NextStepIndex () int`
- `func (*ExecutionContext) Render ()`
- `func (*Executor) ExecuteJob (parentCtx context.Context, execCtx *ExecutionContext) error`
- `func (*LineCapturingWriter) GetLines () []string`
- `func (*LineCapturingWriter) String () string`
- `func (*LineCapturingWriter) Write (p []byte) (int, error)`
- `func (*Linter) Lint () []LintError`
- `func (ExecError) Error () string`
- `func (ExecError) Len () int`

### DefaultOptions

DefaultOptions returns the default executor options.

```go
func DefaultOptions () *Options
```

### DiscoverConfig

DiscoverConfig searches for a config file starting from the given directory,
traversing parent directories until a config file is found or root is reached.
Returns the absolute path to the config file and the directory containing it.

```go
func DiscoverConfig (startDir string) (string, error)
```

### DiscoverConfigFromCwd

DiscoverConfigFromCwd is a convenience wrapper that starts from the current working directory.

```go
func DiscoverConfigFromCwd () (string, error)
```

### EvaluateIf

EvaluateIf evaluates the If condition using expr-lang.
Returns true if the condition is met, false if no condition or condition is false.
Returns error only for invalid expressions.

```go
func EvaluateIf (ctx *ExecutionContext) (bool, error)
```

### ExpandFor

ExpandFor expands a for loop into multiple iteration contexts.
Supports patterns: "item in items" (items is a variable name),
"(index, item) in items", "(key, value) in items",
or any of the above with bash expansion: "item in $(ls ./bin/*.test)".

```go
func ExpandFor (ctx *ExecutionContext, executeCommand func(string) (string, error)) ([]IterationContext, error)
```

### GetDependencies

GetDependencies converts depends_on field (string or []string) to a slice of job names.

```go
func GetDependencies (dependsOn any) []string
```

### InterpolateCommand

InterpolateCommand interpolates a command string.

```go
func InterpolateCommand (cmd string, ctx *ExecutionContext) (string, error)
```

### InterpolateMap

InterpolateMap recursively interpolates all string values in a map.

```go
func InterpolateMap (ctx *ExecutionContext, m map[string]any) error
```

### InterpolateString

InterpolateString replaces ${{ expression }} with values from context.
Supports variable interpolation, dot notation, and expr expressions with ?? and || operators.

```go
func InterpolateString (s string, ctx *ExecutionContext) (string, error)
```

### IsEchoCommand

IsEchoCommand checks if a command is a bare echo command.

```go
func IsEchoCommand (cmd string) bool
```

### ListPipeline

ListPipeline displays a pipeline's job tree with dependencies.

```go
func ListPipeline (pipeline *model.Pipeline) error
```

### LoadPipeline

LoadPipeline loads and parses a pipeline from a yaml file.
Returns the number of documents loaded, the parsed pipeline, and any error.

```go
func LoadPipeline (filePath string) ([]*model.Pipeline, error)
```

### MergeVariables

MergeVariables merges variables from Decl into the execution context.

```go
func MergeVariables (decl *model.Decl, ctx *ExecutionContext) error
```

### NewExec

NewExec creates a new Exec instance.

```go
func NewExec () *Exec
```

### NewExecWithEnv

NewExecWithEnv creates a new Exec instance with environment variables.

```go
func NewExecWithEnv (env map[string]string) *Exec
```

### NewExecutor

NewExecutor creates a new executor with default options.

```go
func NewExecutor () *Executor
```

### NewExecutorWithOptions

NewExecutorWithOptions creates a new executor with custom options.

```go
func NewExecutorWithOptions (opts *Options) *Executor
```

### NewLineCapturingWriter

NewLineCapturingWriter creates a new LineCapturingWriter.

```go
func NewLineCapturingWriter () *LineCapturingWriter
```

### NewLinter

NewLinter creates a new linter.

```go
func NewLinter (pipeline *model.Pipeline) *Linter
```

### ProcessDecl

ProcessDecl processes an Decl and returns a map of variables.
It handles:
- Manual vars with interpolation ($(...), ${{ ... }})
- Include files (.yml format)
Vars take precedence over included files.

```go
func ProcessDecl (decl *model.Decl, ctx *ExecutionContext) (map[string]any, error)
```

### ResolveJobDependencies

ResolveJobDependencies returns jobs in dependency order.
Returns the jobs to run and any resolution errors.

```go
func ResolveJobDependencies (jobs map[string]*model.Job, startingJob string) ([]string, error)
```

### RunPipeline

RunPipeline runs a pipeline with the given options.

```go
func RunPipeline (ctx context.Context, pipeline *model.Pipeline, opts PipelineOptions) error
```

### Sanitize

Sanitize processes raw terminal output and returns clean lines.
It handles:
- Cursor up + clear sequences (\033[nA\033[J) used by treeview
- Carriage returns (\r) by taking content after the last \r
- CRLF normalization
- Preserves ANSI color sequences in output

Returns sanitized lines with colors preserved.

```go
func Sanitize (in string) ([]string, error)
```

### StripANSI

StripANSI removes all ANSI escape sequences from a string.

```go
func StripANSI (in string) string
```

### ValidateJobRequirements

ValidateJobRequirements checks that all required variables are present in the context.
Returns an error with a clear message listing missing variables.

```go
func ValidateJobRequirements (job *model.Job, ctx *ExecutionContext) error
```

### VisualLength

VisualLength returns the visual length of a string (excluding ANSI sequences).

```go
func VisualLength (s string) int
```

### ExecuteCommand

ExecuteCommand will run the command quietly.

```go
func (*Exec) ExecuteCommand (cmdStr string) (string, error)
```

### ExecuteCommandWithQuiet

ExecuteCommandWithQuiet executes a shell command with quiet mode.

```go
func (*Exec) ExecuteCommandWithQuiet (cmdStr string, verbose bool) (string, error)
```

### ExecuteCommandWithQuietAndCapture

ExecuteCommandWithQuietAndCapture executes a shell command with quiet mode and captures stderr.
Returns (stdout, error). If error occurs, stderr is logged to the global buffer.

```go
func (*Exec) ExecuteCommandWithQuietAndCapture (cmdStr string, verbose bool) (string, error)
```

### ExecuteCommandWithWriter

ExecuteCommandWithWriter executes a command and writes output to the provided writer.
If usePTY is true, allocates a PTY for the command (enables colored output for tools like gotestsum).
Also returns the full stdout string for the caller.

```go
func (*Exec) ExecuteCommandWithWriter (writer io.Writer, cmdStr string, usePTY bool) (string, error)
```

### Copy

Copy copies everything except Context. Variables are shallow-copied.

```go
func (*ExecutionContext) Copy () *ExecutionContext
```

### NextStepIndex

NextStepIndex returns the next sequential step index for this job execution.
This ensures each step/iteration gets a unique number.

```go
func (*ExecutionContext) NextStepIndex () int
```

### Render

Render refreshes the treeview.

```go
func (*ExecutionContext) Render ()
```

### ExecuteJob

ExecuteJob runs a single job.

```go
func (*Executor) ExecuteJob (parentCtx context.Context, execCtx *ExecutionContext) error
```

### GetLines

GetLines returns all captured output as lines.

```go
func (*LineCapturingWriter) GetLines () []string
```

### String

String returns the raw captured output.

```go
func (*LineCapturingWriter) String () string
```

### Write

Write implements io.Writer.

```go
func (*LineCapturingWriter) Write (p []byte) (int, error)
```

### Lint

Lint validates the pipeline and returns any errors.

```go
func (*Linter) Lint () []LintError
```

### Error

Error returns the error message.

```go
func (ExecError) Error () string
```

### Len

Len returns the length of the error message.

```go
func (ExecError) Len () int
```


