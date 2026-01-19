package eventlog

import "time"

// Result represents the final outcome of an execution.
type Result string

const (
	ResultPass    Result = "pass"
	ResultFail    Result = "fail"
	ResultSkipped Result = "skipped"
)

// Event represents a single execution event in the log (one per exec).
type Event struct {
	ID          string  `yaml:"id"`
	Run         string  `yaml:"run"`
	Result      Result  `yaml:"result"`
	Start       float64 `yaml:"start"`                  // Seconds since run started
	Duration    float64 `yaml:"duration"`               // Seconds
	Error       string  `yaml:"error,omitempty"`        // Only for fail events
	GoroutineID uint64  `yaml:"goroutine_id,omitempty"` // Only when debug is enabled
}

// StateNode represents a node in the execution state tree for YAML output.
type StateNode struct {
	Name      string       `yaml:"name"`
	ID        string       `yaml:"id,omitempty"`
	Status    string       `yaml:"status"` // Readable string: pending, running, passed, failed, skipped, conditional
	Result    Result       `yaml:"result,omitempty"`
	If        string       `yaml:"if,omitempty"` // Condition that was evaluated
	CreatedAt time.Time    `yaml:"created_at"`
	UpdatedAt time.Time    `yaml:"updated_at,omitempty"`
	Start     float64      `yaml:"start,omitempty"`    // Seconds offset from run start
	Duration  float64      `yaml:"duration,omitempty"` // Total duration in seconds
	Steps     int          `yaml:"steps,omitempty"`    // Number of steps executed (for jobs/workflow)
	Children  []*StateNode `yaml:"children,omitempty"`
}

// RunMetadata contains information about the execution environment.
type RunMetadata struct {
	RunID      string    `yaml:"run_id"`
	CreatedAt  time.Time `yaml:"created_at"`
	Pipeline   string    `yaml:"pipeline,omitempty"`
	File       string    `yaml:"file,omitempty"`
	ModulePath string    `yaml:"module_path,omitempty"`
	Git        *GitInfo  `yaml:"git,omitempty"`
}

// GitInfo contains git repository information.
type GitInfo struct {
	Commit     string `yaml:"commit,omitempty"`
	Branch     string `yaml:"branch,omitempty"`
	RemoteURL  string `yaml:"remote_url,omitempty"`
	Repository string `yaml:"repository,omitempty"` // Extracted from remote URL
}

// Log is the complete log structure written to YAML.
type Log struct {
	Metadata RunMetadata `yaml:"metadata"`
	State    *StateNode  `yaml:"state"`
	Events   []*Event    `yaml:"events"`
	Summary  *RunSummary `yaml:"summary,omitempty"`
}

// RunSummary provides aggregate statistics for the run.
type RunSummary struct {
	Duration     float64 `yaml:"duration"`               // Total duration in seconds
	TotalSteps   int     `yaml:"total_steps"`            // Total steps executed
	PassedSteps  int     `yaml:"passed_steps"`           // Steps that passed
	FailedSteps  int     `yaml:"failed_steps"`           // Steps that failed
	SkippedSteps int     `yaml:"skipped_steps"`          // Steps that were skipped
	Result       Result  `yaml:"result"`                 // Overall result
	MemoryAlloc  uint64  `yaml:"memory_alloc,omitempty"` // Memory allocated in bytes
	Goroutines   int     `yaml:"goroutines,omitempty"`   // Number of goroutines running
}
