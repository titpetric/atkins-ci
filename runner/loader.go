package runner

import (
	"fmt"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/titpetric/atkins/model"
)

// LoadPipeline loads and parses a pipeline from a yaml file.
// Returns the number of documents loaded, the parsed pipeline, and any error.
func LoadPipeline(filePath string) ([]*model.Pipeline, error) {
	// Read the raw file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline file: %w", err)
	}

	// Parse with plain YAML first (no expression evaluation)
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))

	result := []*model.Pipeline{
		{},
	}
	if err := decoder.Decode(result[0]); err != nil {
		return nil, fmt.Errorf("error decoding pipeline: %w", err)
	}

	for jobName, job := range result[0].Jobs {
		job.Name = jobName
		if strings.Contains(jobName, ":") {
			job.Nested = true
		}
	}

	for taskName, task := range result[0].Tasks {
		task.Name = taskName
		if strings.Contains(taskName, ":") {
			task.Nested = true
		}
	}

	return result, nil
}
