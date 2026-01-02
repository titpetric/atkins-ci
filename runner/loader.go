package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/yamlexpr"
	"gopkg.in/yaml.v3"
)

// LoadPipeline loads and parses a pipeline from a yaml file using yamlexpr
// Returns the number of documents loaded, the parsed pipeline, and any error
func LoadPipeline(filePath string) ([]*model.Pipeline, error) {
	// Get the directory of the file for relative path resolution
	dir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	// Create yamlexpr evaluator with the file's directory as root
	expr := yamlexpr.New(os.DirFS(dir))

	// Load the file using yamlexpr (without interpolation)
	docs, err := expr.Load(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline file: %w", err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents found in pipeline file")
	}

	result := make([]*model.Pipeline, 0, len(docs))

	for _, doc := range docs {
		// Flatten matrix-expanded jobs in the document first
		if err := flattenMatrixJobsInDocument(doc); err != nil {
			return nil, fmt.Errorf("failed to flatten matrix jobs: %w", err)
		}

		// Convert document to pipeline structure
		pipeline := &model.Pipeline{}

		// Marshal and unmarshal through YAML to convert Document to Pipeline
		data, err := yaml.Marshal(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal document: %w", err)
		}
		if err := yaml.Unmarshal(data, pipeline); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pipeline: %w", err)
		}

		result = append(result, pipeline)
	}

	return result, nil
}

// flattenMatrixJobsInDocument converts matrix-expanded job arrays into individual jobs
func flattenMatrixJobsInDocument(doc yamlexpr.Document) error {
	jobsVal, ok := doc["jobs"]
	if !ok {
		return nil
	}

	jobsMap, ok := jobsVal.(map[string]interface{})
	if !ok {
		return nil
	}

	// Iterate through jobs and flatten arrays (matrix-expanded)
	flatJobs := make(map[string]interface{})

	for jobName, jobVal := range jobsMap {
		// If the job is an array, it means it was expanded by matrix
		if jobSlice, ok := jobVal.([]interface{}); ok {
			// Create individual jobs for each matrix combination
			for i, jobComb := range jobSlice {
				combinedName := fmt.Sprintf("%s[%d]", jobName, i)
				flatJobs[combinedName] = jobComb
			}
		} else {
			// Keep non-matrix jobs as-is
			flatJobs[jobName] = jobVal
		}
	}

	doc["jobs"] = flatJobs
	return nil
}
