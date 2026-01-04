package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/yamlexpr"
	"gopkg.in/yaml.v3"
)

// LoadPipeline loads and parses a pipeline from a yaml file
// Returns the number of documents loaded, the parsed pipeline, and any error
func LoadPipeline(filePath string) ([]*model.Pipeline, error) {
	// Get the directory of the file for relative path resolution
	dir := filepath.Dir(filePath)

	// Read the raw file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline file: %w", err)
	}

	// Parse with plain YAML first (no expression evaluation)
	var docInterface interface{}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	docs := []yamlexpr.Document{}

	for {
		if err := decoder.Decode(&docInterface); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode yaml: %w", err)
		}
		if docInterface == nil {
			continue
		}

		// Convert to yamlexpr.Document
		doc, ok := docInterface.(map[string]interface{})
		if !ok {
			continue
		}

		docs = append(docs, yamlexpr.Document(doc))
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

		// Expand glob patterns in job vars
		if err := expandGlobInJobVars(doc, dir); err != nil {
			return nil, fmt.Errorf("failed to expand glob in vars: %w", err)
		}

		// Expand for loops in steps
		if err := expandForLoopsInSteps(doc); err != nil {
			return nil, fmt.Errorf("failed to expand for loops: %w", err)
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

// expandGlobInJobVars expands glob patterns in job vars sections
// Supports vars with a "glob" field that gets expanded to a list
func expandGlobInJobVars(doc yamlexpr.Document, baseDir string) error {
	jobsVal, ok := doc["jobs"]
	if !ok {
		return nil
	}

	jobsMap, ok := jobsVal.(map[string]interface{})
	if !ok {
		return nil
	}

	for _, jobVal := range jobsMap {
		jobMap, ok := jobVal.(map[string]interface{})
		if !ok {
			continue
		}

		varsVal, ok := jobMap["vars"]
		if !ok {
			continue
		}

		varsMap, ok := varsVal.(map[string]interface{})
		if !ok {
			continue
		}

		// Process each variable for glob patterns
		for varName, varVal := range varsMap {
			// Check if this is a map with a "glob" field
			if varValMap, ok := varVal.(map[string]interface{}); ok {
				if globPattern, ok := varValMap["glob"].(string); ok {
					// Resolve pattern relative to base directory if not absolute
					pattern := globPattern
					if !filepath.IsAbs(pattern) {
						pattern = filepath.Join(baseDir, pattern)
					}

					// Expand the glob pattern
					files, err := filepath.Glob(pattern)
					if err != nil {
						return fmt.Errorf("glob pattern error for %s: %w", globPattern, err)
					}

					sort.Strings(files)

					// Convert absolute paths back to relative if they started relative
					if !filepath.IsAbs(globPattern) {
						relFiles := make([]string, len(files))
						for i, f := range files {
							rel, err := filepath.Rel(baseDir, f)
							if err != nil {
								relFiles[i] = f
							} else {
								relFiles[i] = rel
							}
						}
						files = relFiles
					}

					// Replace the var value with the expanded list
					result := make([]interface{}, len(files))
					for i, f := range files {
						result[i] = f
					}
					varsMap[varName] = result
				}
			}
		}
	}

	return nil
}

// expandForLoopsInSteps expands for loops in step arrays
// A step with "for: item in items" and "run: cmd" becomes multiple steps
func expandForLoopsInSteps(doc yamlexpr.Document) error {
	jobsVal, ok := doc["jobs"]
	if !ok {
		return nil
	}

	jobsMap, ok := jobsVal.(map[string]interface{})
	if !ok {
		return nil
	}

	for _, jobVal := range jobsMap {
		jobMap, ok := jobVal.(map[string]interface{})
		if !ok {
			continue
		}

		stepsVal, ok := jobMap["steps"]
		if !ok {
			continue
		}

		stepsSlice, ok := stepsVal.([]interface{})
		if !ok {
			continue
		}

		// Process steps for for loops
		expandedSteps := make([]interface{}, 0, len(stepsSlice))

		for _, stepVal := range stepsSlice {
			stepMap, ok := stepVal.(map[string]interface{})
			if !ok {
				expandedSteps = append(expandedSteps, stepVal)
				continue
			}

			// Check if this step has a "for" field
			forField, hasFor := stepMap["for"].(string)
			if !hasFor {
				expandedSteps = append(expandedSteps, stepVal)
				continue
			}

			// Parse the for loop: "item in items"
			parts := strings.Split(forField, " in ")
			if len(parts) != 2 {
				return fmt.Errorf("invalid for loop syntax: %q, expected 'item in items'", forField)
			}

			itemVar := strings.TrimSpace(parts[0])
			itemsVar := strings.TrimSpace(parts[1])

			// Get the items from vars (we'll resolve this during execution)
			// For now, we need to fetch it from the job's vars
			varsVal, ok := jobMap["vars"]
			if !ok {
				return fmt.Errorf("for loop references items %q but vars is not defined", itemsVar)
			}

			varsMap, ok := varsVal.(map[string]interface{})
			if !ok {
				return fmt.Errorf("vars is not a map")
			}

			items, ok := varsMap[itemsVar]
			if !ok {
				return fmt.Errorf("for loop references items %q which is not defined in vars", itemsVar)
			}

			// Convert items to slice
			itemsSlice, ok := items.([]interface{})
			if !ok {
				return fmt.Errorf("items %q is not a list", itemsVar)
			}

			// Create a step for each item
			for _, item := range itemsSlice {
				itemStr := fmt.Sprintf("%v", item)

				// Create a new step by copying the for loop step and replacing the item variable
				newStep := make(map[string]interface{})
				for k, v := range stepMap {
					if k == "for" {
						// Skip the for field in the expanded step
						continue
					}

					// Interpolate the item variable in string values
					if strVal, ok := v.(string); ok {
						// Replace ${item} with the actual item value
						expanded := strings.ReplaceAll(strVal, "${"+itemVar+"}", itemStr)
						newStep[k] = expanded
					} else {
						newStep[k] = v
					}
				}

				// Set the step name if not already set
				if _, hasName := newStep["name"]; !hasName {
					if runCmd, ok := newStep["run"].(string); ok {
						newStep["name"] = runCmd
					}
				}

				expandedSteps = append(expandedSteps, newStep)
			}
		}

		jobMap["steps"] = expandedSteps
	}

	return nil
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
