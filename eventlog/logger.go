package eventlog

import (
	"bytes"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	ulid "github.com/oklog/ulid/v2"
	yaml "gopkg.in/yaml.v3"
)

// RuntimeStats holds memory and goroutine statistics.
type RuntimeStats struct {
	MemoryAlloc uint64
	Goroutines  int
}

// CaptureRuntimeStats captures current memory allocation and goroutine count.
func CaptureRuntimeStats() RuntimeStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return RuntimeStats{
		MemoryAlloc: m.Alloc,
		Goroutines:  runtime.NumGoroutine(),
	}
}

// Logger collects events during execution and writes the final log.
type Logger struct {
	mu        sync.Mutex
	filePath  string
	metadata  RunMetadata
	events    []*Event
	startTime time.Time
	debug     bool
}

// NewLogger creates a new event logger.
// If filePath is empty, returns nil (no logging occurs).
func NewLogger(filePath, pipelineName, pipelineFile string, debug bool) *Logger {
	if filePath == "" {
		return nil
	}

	now := time.Now()
	runID := ulid.Make().String()

	metadata := RunMetadata{
		RunID:     runID,
		CreatedAt: now,
		Pipeline:  pipelineName,
		File:      pipelineFile,
	}

	// Capture git info
	metadata.Git = CaptureGitInfo()

	// Capture module path
	metadata.ModulePath = CaptureModulePath()

	return &Logger{
		filePath:  filePath,
		metadata:  metadata,
		events:    make([]*Event, 0),
		startTime: now,
		debug:     debug,
	}
}

// LogExec logs a single execution event (one per exec).
func (l *Logger) LogExec(result Result, id, run string, start float64, durationMs int64, err error) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	event := &Event{
		ID:       id,
		Run:      run,
		Result:   result,
		Start:    start,
		Duration: float64(durationMs) / 1000.0,
		Error:    errMsg,
	}
	if l.debug {
		event.GoroutineID = getGoroutineID()
	}
	l.events = append(l.events, event)
}

// elapsed returns seconds since the logger started.
func (l *Logger) elapsed() float64 {
	return time.Since(l.startTime).Seconds()
}

// Write writes the final event log to the file.
func (l *Logger) Write(state *StateNode, summary *RunSummary) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	log := &Log{
		Metadata: l.metadata,
		State:    state,
		Events:   l.events,
		Summary:  summary,
	}

	data, err := yaml.Marshal(log)
	if err != nil {
		return err
	}

	return os.WriteFile(l.filePath, data, 0o644)
}

// GetStartTime returns the start time of the run.
func (l *Logger) GetStartTime() time.Time {
	if l == nil {
		return time.Time{}
	}
	return l.startTime
}

// GetElapsed returns the current elapsed time in seconds.
func (l *Logger) GetElapsed() float64 {
	if l == nil {
		return 0
	}
	return l.elapsed()
}

// GetEvents returns a copy of the events slice.
func (l *Logger) GetEvents() []*Event {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	events := make([]*Event, len(l.events))
	copy(events, l.events)
	return events
}

// getGoroutineID extracts the goroutine ID from the runtime stack.
// This is a common hack since Go doesn't expose goroutine IDs directly.
func getGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Stack output starts with "goroutine <id> ["
	fields := bytes.Fields(buf[:n])
	if len(fields) < 2 {
		return 0
	}
	id, _ := strconv.ParseUint(string(fields[1]), 10, 64)
	return id
}
