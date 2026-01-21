package eventlog

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

func TestNewLogger_NilWhenEmpty(t *testing.T) {
	logger := NewLogger("", "test-pipeline", "test.yml", false)
	assert.Nil(t, logger)
}

func TestNewLogger_CreatesLogger(t *testing.T) {
	tmpFile := "test_eventlog.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)
	assert.NotEmpty(t, logger.metadata.RunID)
	assert.Equal(t, "test-pipeline", logger.metadata.Pipeline)
	assert.Equal(t, "test.yml", logger.metadata.File)
}

func TestLogger_LogExec_Pass(t *testing.T) {
	tmpFile := "test_pass.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)

	logger.LogExec(ResultPass, "jobs.test-job.steps.0", "echo hello", 0.5, 100, nil)

	events := logger.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "jobs.test-job.steps.0", events[0].ID)
	assert.Equal(t, "echo hello", events[0].Run)
	assert.Equal(t, ResultPass, events[0].Result)
	assert.Equal(t, 0.5, events[0].Start)
	assert.Equal(t, 0.1, events[0].Duration)
	assert.Empty(t, events[0].Error)
}

func TestLogger_LogExec_Fail(t *testing.T) {
	tmpFile := "test_fail.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)

	testErr := assert.AnError
	logger.LogExec(ResultFail, "jobs.test-job.steps.0", "bad command", 1.0, 150, testErr)

	events := logger.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "jobs.test-job.steps.0", events[0].ID)
	assert.Equal(t, "bad command", events[0].Run)
	assert.Equal(t, ResultFail, events[0].Result)
	assert.Equal(t, 1.0, events[0].Start)
	assert.Equal(t, 0.15, events[0].Duration)
	assert.Contains(t, events[0].Error, "assert.AnError")
}

func TestLogger_LogExec_Skip(t *testing.T) {
	tmpFile := "test_skip.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)

	logger.LogExec(ResultSkipped, "jobs.test-job.steps.0", "skipped step", 2.0, 0, nil)

	events := logger.GetEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "jobs.test-job.steps.0", events[0].ID)
	assert.Equal(t, "skipped step", events[0].Run)
	assert.Equal(t, ResultSkipped, events[0].Result)
	assert.Equal(t, 2.0, events[0].Start)
	assert.Equal(t, 0.0, events[0].Duration)
}

func TestLogger_NilSafe(t *testing.T) {
	var logger *Logger

	// All methods should be safe to call on nil
	logger.LogExec(ResultPass, "id", "run", 0, 100, nil)
	assert.Nil(t, logger.GetEvents())
	assert.Equal(t, float64(0), logger.GetElapsed())
	assert.Equal(t, time.Time{}, logger.GetStartTime())
	assert.NoError(t, logger.Write(nil, nil))
}

func TestLogger_Write(t *testing.T) {
	tmpFile := "test_write.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)

	logger.LogExec(ResultPass, "jobs.test-job.steps.0", "echo hello", 0.1, 100, nil)

	state := &StateNode{
		Name:      "test-pipeline",
		Status:    "passed",
		Result:    ResultPass,
		CreatedAt: time.Now(),
		Duration:  0.5,
	}

	summary := &RunSummary{
		Duration:     0.5,
		TotalSteps:   1,
		PassedSteps:  1,
		FailedSteps:  0,
		SkippedSteps: 0,
		Result:       ResultPass,
	}

	err := logger.Write(state, summary)
	require.NoError(t, err)

	// Read and verify the file
	data, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	var log Log
	err = yaml.Unmarshal(data, &log)
	require.NoError(t, err)

	assert.NotEmpty(t, log.Metadata.RunID)
	assert.Equal(t, "test-pipeline", log.Metadata.Pipeline)
	assert.Equal(t, "test-pipeline", log.State.Name)
	assert.Len(t, log.Events, 1)
	assert.Equal(t, ResultPass, log.Summary.Result)
}

func TestLogger_GetElapsed(t *testing.T) {
	tmpFile := "test_elapsed.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)

	time.Sleep(10 * time.Millisecond)

	elapsed := logger.GetElapsed()
	assert.Greater(t, elapsed, 0.0)
}

func TestLogger_DebugGoroutineID(t *testing.T) {
	tmpFile := "test_debug.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", true)
	require.NotNil(t, logger)

	logger.LogExec(ResultPass, "jobs.test-job.steps.0", "echo hello", 0.1, 100, nil)

	events := logger.GetEvents()
	require.Len(t, events, 1)

	// Goroutine ID should be non-zero when debug is enabled
	assert.Greater(t, events[0].GoroutineID, uint64(0))
}

func TestLogger_NoGoroutineIDWithoutDebug(t *testing.T) {
	tmpFile := "test_nodebug.yml"
	defer os.Remove(tmpFile)

	logger := NewLogger(tmpFile, "test-pipeline", "test.yml", false)
	require.NotNil(t, logger)

	logger.LogExec(ResultPass, "jobs.test-job.steps.0", "echo hello", 0.1, 100, nil)

	events := logger.GetEvents()
	require.Len(t, events, 1)

	// Goroutine ID should be zero when debug is disabled
	assert.Equal(t, uint64(0), events[0].GoroutineID)
}

func TestGetGoroutineID(t *testing.T) {
	id := getGoroutineID()
	assert.Greater(t, id, uint64(0))
}
