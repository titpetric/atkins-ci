package runner

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// StepLogger wraps slog.Logger for logging step execution.
type StepLogger struct {
	logger       *slog.Logger
	pipelineName string
	mu           sync.Mutex
}

// NewStepLoggerWithPipeline creates a new step logger writing to the given file.
// If filePath is empty, no logging occurs.
func NewStepLoggerWithPipeline(filePath string, pipelineName string) (*StepLogger, error) {
	if filePath == "" {
		return &StepLogger{logger: nil, pipelineName: pipelineName}, nil
	}

	// Open or create log file in append mode
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	// Create text handler with custom formatting
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})

	logger := slog.New(handler)
	return &StepLogger{logger: logger, pipelineName: pipelineName}, nil
}

// NewStepLogger creates a new step logger writing to the given file (for backward compatibility).
// If filePath is empty, no logging occurs.
func NewStepLogger(filePath string) (*StepLogger, error) {
	return NewStepLoggerWithPipeline(filePath, "")
}

// LogRun logs a RUN event for a step.
func (sl *StepLogger) LogRun(jobName string, stepIndex int, stepName string) {
	if sl.logger == nil {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	stepID := generateStepID(jobName, stepIndex)
	sl.logger.Info("RUN",
		slog.String("pipeline", sl.pipelineName),
		slog.String("id", stepID),
		slog.String("name", stepName),
		slog.Int("sequence", stepIndex), // Add sequence as additional metadata
	)
}

// LogPass logs a PASS event for a step.
func (sl *StepLogger) LogPass(jobName string, stepIndex int, stepName string, duration int64) {
	if sl.logger == nil {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	stepID := generateStepID(jobName, stepIndex)
	durationStr := fmt.Sprintf("%.4f", float64(duration)/1000.0)
	sl.logger.Info("PASS",
		slog.String("pipeline", sl.pipelineName),
		slog.String("id", stepID),
		slog.String("duration", durationStr),
		slog.Int("sequence", stepIndex), // Add sequence as additional metadata
	)
}

// LogFail logs a FAIL event for a step.
func (sl *StepLogger) LogFail(jobName string, stepIndex int, stepName string, err error, duration int64) {
	if sl.logger == nil {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	stepID := generateStepID(jobName, stepIndex)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	durationStr := fmt.Sprintf("%.4f", float64(duration)/1000.0)

	sl.logger.Info("FAIL",
		slog.String("pipeline", sl.pipelineName),
		slog.String("id", stepID),
		slog.String("error", errMsg),
		slog.String("duration", durationStr),
		slog.Int("sequence", stepIndex), // Add sequence as additional metadata
	)
}

// LogSkip logs a SKIP event for a step.
func (sl *StepLogger) LogSkip(jobName string, stepIndex int, stepName string) {
	if sl.logger == nil {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	stepID := generateStepID(jobName, stepIndex)
	sl.logger.Info("SKIP",
		slog.String("pipeline", sl.pipelineName),
		slog.String("id", stepID),
		slog.Int("sequence", stepIndex), // Add sequence as additional metadata
	)
}

// LogOutput logs command output for a step.
func (sl *StepLogger) LogOutput(jobName string, stepIndex int, output string) {
	if sl.logger == nil || output == "" {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	stepID := generateStepID(jobName, stepIndex)
	sl.logger.Info("OUTPUT",
		slog.String("pipeline", sl.pipelineName),
		slog.String("id", stepID),
		slog.String("output", output),
	)
}

// generateStepID creates a step ID from job name and sequential step index
// Format follows GitHub Actions: jobs.<jobName>.steps.<sequentialIndex>
func generateStepID(jobName string, stepIndex int) string {
	if jobName == "" {
		return ""
	}
	// Format: jobs.<jobName>.steps.<sequentialIndex>
	return "jobs." + jobName + ".steps." + fmt.Sprintf("%d", stepIndex)
}
