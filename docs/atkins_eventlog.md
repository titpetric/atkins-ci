# Package ./eventlog

```go
import (
	"github.com/titpetric/atkins/eventlog"
}
```

## Types

```go
// Event represents a single execution event in the log (one per exec).
type Event struct {
	ID		string	`yaml:"id"`
	Run		string	`yaml:"run"`
	Result		Result	`yaml:"result"`
	Start		float64	`yaml:"start"`			// Seconds since run started
	Duration	float64	`yaml:"duration"`		// Seconds
	Error		string	`yaml:"error,omitempty"`	// Only for fail events
	GoroutineID	uint64	`yaml:"goroutine_id,omitempty"`	// Only when debug is enabled
}
```

```go
// GitInfo contains git repository information.
type GitInfo struct {
	Commit		string	`yaml:"commit,omitempty"`
	Branch		string	`yaml:"branch,omitempty"`
	RemoteURL	string	`yaml:"remote_url,omitempty"`
	Repository	string	`yaml:"repository,omitempty"`	// Extracted from remote URL
}
```

```go
// Log is the complete log structure written to YAML.
type Log struct {
	Metadata	RunMetadata	`yaml:"metadata"`
	State		*StateNode	`yaml:"state"`
	Events		[]*Event	`yaml:"events"`
	Summary		*RunSummary	`yaml:"summary,omitempty"`
}
```

```go
// Logger collects events during execution and writes the final log.
type Logger struct {
	mu		sync.Mutex
	filePath	string
	metadata	RunMetadata
	events		[]*Event
	startTime	time.Time
	debug		bool
}
```

```go
// Result represents the final outcome of an execution.
type Result string
```

```go
// RunMetadata contains information about the execution environment.
type RunMetadata struct {
	RunID		string		`yaml:"run_id"`
	CreatedAt	time.Time	`yaml:"created_at"`
	Pipeline	string		`yaml:"pipeline,omitempty"`
	File		string		`yaml:"file,omitempty"`
	ModulePath	string		`yaml:"module_path,omitempty"`
	Git		*GitInfo	`yaml:"git,omitempty"`
}
```

```go
// RunSummary provides aggregate statistics for the run.
type RunSummary struct {
	Duration	float64	`yaml:"duration"`		// Total duration in seconds
	TotalSteps	int	`yaml:"total_steps"`		// Total steps executed
	PassedSteps	int	`yaml:"passed_steps"`		// Steps that passed
	FailedSteps	int	`yaml:"failed_steps"`		// Steps that failed
	SkippedSteps	int	`yaml:"skipped_steps"`		// Steps that were skipped
	Result		Result	`yaml:"result"`			// Overall result
	MemoryAlloc	uint64	`yaml:"memory_alloc,omitempty"`	// Memory allocated in bytes
	Goroutines	int	`yaml:"goroutines,omitempty"`	// Number of goroutines running
}
```

```go
// RuntimeStats holds memory and goroutine statistics.
type RuntimeStats struct {
	MemoryAlloc	uint64
	Goroutines	int
}
```

```go
// StateNode represents a node in the execution state tree for YAML output.
type StateNode struct {
	Name		string		`yaml:"name"`
	ID		string		`yaml:"id,omitempty"`
	Status		string		`yaml:"status"`	// Readable string: pending, running, passed, failed, skipped, conditional
	Result		Result		`yaml:"result,omitempty"`
	If		string		`yaml:"if,omitempty"`	// Condition that was evaluated
	CreatedAt	time.Time	`yaml:"created_at"`
	UpdatedAt	time.Time	`yaml:"updated_at,omitempty"`
	Start		float64		`yaml:"start,omitempty"`	// Seconds offset from run start
	Duration	float64		`yaml:"duration,omitempty"`	// Total duration in seconds
	Steps		int		`yaml:"steps,omitempty"`	// Number of steps executed (for jobs/workflow)
	Children	[]*StateNode	`yaml:"children,omitempty"`
}
```

## Consts

```go
// Result constants for execution outcomes.
const (
	ResultPass	Result	= "pass"
	ResultFail	Result	= "fail"
	ResultSkipped	Result	= "skipped"
)
```

## Function symbols

- `func CalculateDuration (node *StateNode) float64`
- `func CaptureGitInfo () *GitInfo`
- `func CaptureModulePath () string`
- `func CaptureRuntimeStats () RuntimeStats`
- `func CountSteps (node *StateNode) int`
- `func NewLogger (filePath,pipelineName,pipelineFile string, debug bool) *Logger`
- `func NodeToStateNode (node *treeview.Node) *StateNode`
- `func TreeNodeToStateNode (node *treeview.TreeNode) *StateNode`
- `func (*Logger) GetElapsed () float64`
- `func (*Logger) GetEvents () []*Event`
- `func (*Logger) GetStartTime () time.Time`
- `func (*Logger) LogExec (result Result, id,run string, start float64, durationMs int64, err error)`
- `func (*Logger) Write (state *StateNode, summary *RunSummary) error`

### CalculateDuration

CalculateDuration calculates the total duration from node timing.

```go
func CalculateDuration (node *StateNode) float64
```

### CaptureGitInfo

CaptureGitInfo captures git repository information.

```go
func CaptureGitInfo () *GitInfo
```

### CaptureModulePath

CaptureModulePath captures the Go module path from go.mod if present.

```go
func CaptureModulePath () string
```

### CaptureRuntimeStats

CaptureRuntimeStats captures current memory allocation and goroutine count.

```go
func CaptureRuntimeStats () RuntimeStats
```

### CountSteps

CountSteps counts steps by result in a StateNode tree.

```go
func CountSteps (node *StateNode) int
```

### NewLogger

NewLogger creates a new event logger.
If filePath is empty, returns nil (no logging occurs).

```go
func NewLogger (filePath,pipelineName,pipelineFile string, debug bool) *Logger
```

### NodeToStateNode

NodeToStateNode converts a treeview.Node to a StateNode for serialization.

```go
func NodeToStateNode (node *treeview.Node) *StateNode
```

### TreeNodeToStateNode

TreeNodeToStateNode converts a treeview.TreeNode to a StateNode.

```go
func TreeNodeToStateNode (node *treeview.TreeNode) *StateNode
```

### GetElapsed

GetElapsed returns the current elapsed time in seconds.

```go
func (*Logger) GetElapsed () float64
```

### GetEvents

GetEvents returns a copy of the events slice.

```go
func (*Logger) GetEvents () []*Event
```

### GetStartTime

GetStartTime returns the start time of the run.

```go
func (*Logger) GetStartTime () time.Time
```

### LogExec

LogExec logs a single execution event (one per exec).

```go
func (*Logger) LogExec (result Result, id,run string, start float64, durationMs int64, err error)
```

### Write

Write writes the final event log to the file.

```go
func (*Logger) Write (state *StateNode, summary *RunSummary) error
```


