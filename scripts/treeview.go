//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type Result string

type StateNode struct {
	Name     string       `yaml:"name"`
	ID       string       `yaml:"id,omitempty"`
	Status   string       `yaml:"status"`
	Result   Result       `yaml:"result,omitempty"`
	Children []*StateNode `yaml:"children,omitempty"`
}

type RunMetadata struct {
	Pipeline string `yaml:"pipeline,omitempty"`
}

type Log struct {
	Metadata RunMetadata `yaml:"metadata"`
	State    *StateNode  `yaml:"state"`
}

type Entity struct {
	ID        string
	Label     string
	Type      string // "pipeline", "job", "step", "task"
	ExecCount int
}

type Relation struct {
	From  string
	To    string
	Label string
}

// StepResult tracks IDs for multi-link chaining
type StepResult struct {
	StepID     string // The step/task itself
	ExitID     string // Exit point (last step of invoked job)
	InvokedJob string // The invoked job ID (for task invocations)
}

var (
	entities  = make(map[string]*Entity)
	relations []Relation
	taskRegex = regexp.MustCompile(`^task:\s*(.+)$`)
	runRegex  = regexp.MustCompile(`^run:\s*(.+)$`)
)

func main() {
	inputFile := flag.String("i", "atkins.log", "Input atkins log file (YAML)")
	flag.Parse()

	data, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var log Log
	if err := yaml.Unmarshal(data, &log); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	if log.State == nil {
		fmt.Fprintf(os.Stderr, "No state found in log\n")
		os.Exit(1)
	}

	// Process the tree
	pipelineID := sanitizeID(log.State.Name)
	entities[pipelineID] = &Entity{
		ID:    pipelineID,
		Label: log.State.Name,
		Type:  "pipeline",
	}

	// Process pipeline children (jobs)
	var prevJobID string
	for _, jobNode := range log.State.Children {
		jobID := processJob(pipelineID, jobNode, prevJobID)
		prevJobID = jobID
	}

	// Output PlantUML
	outputPlantUML(log.Metadata.Pipeline)
}

func processJob(pipelineID string, node *StateNode, prevJobID string) string {
	jobID := sanitizeID(node.Name)

	entities[jobID] = &Entity{
		ID:    jobID,
		Label: node.Name,
		Type:  "job",
	}

	// Pipeline -> Job
	relations = append(relations, Relation{From: pipelineID, To: jobID, Label: "job"})

	// Chain jobs: prevJob -> thisJob
	if prevJobID != "" {
		relations = append(relations, Relation{From: prevJobID, To: jobID, Label: "next"})
	}

	// Process children (steps or tasks)
	processSteps(jobID, node.Children)

	return jobID
}

// processSteps returns the exit point (last step ID) for chaining
func processSteps(parentID string, children []*StateNode) string {
	var prev *StepResult
	stepIndex := 0

	for _, child := range children {
		result := processStep(parentID, child, stepIndex, prev)
		if result != nil {
			prev = result
			stepIndex++
		}
	}
	if prev != nil {
		return prev.ExitID
	}
	return ""
}

func processStep(parentID string, node *StateNode, index int, prev *StepResult) *StepResult {
	name := node.Name

	// Helper to add links from previous step/task/job to current
	addPrevLinks := func(toID string) {
		if prev == nil {
			relations = append(relations, Relation{From: parentID, To: toID, Label: "step.0"})
		} else {
			// Link from previous task
			relations = append(relations, Relation{From: prev.StepID, To: toID, Label: fmt.Sprintf("step.%d", index)})
			// Link from invoked job (if any)
			if prev.InvokedJob != "" {
				relations = append(relations, Relation{From: prev.InvokedJob, To: toID, Label: "next"})
			}
			// Link from exit point (last step of invoked job)
			if prev.ExitID != prev.StepID && prev.ExitID != prev.InvokedJob {
				relations = append(relations, Relation{From: prev.ExitID, To: toID, Label: "completes"})
			}
		}
	}

	// Check if this is a task invocation
	if matches := taskRegex.FindStringSubmatch(name); len(matches) > 1 {
		taskName := matches[1]
		taskID := fmt.Sprintf("%s_task_%s", parentID, sanitizeID(taskName))

		entities[taskID] = &Entity{
			ID:    taskID,
			Label: fmt.Sprintf("task: %s", taskName),
			Type:  "task",
		}

		addPrevLinks(taskID)

		// Process task's children (the actual job invocation)
		result := &StepResult{StepID: taskID, ExitID: taskID}
		if len(node.Children) > 0 {
			for _, taskChild := range node.Children {
				// This is the invoked job
				invokedJobID := sanitizeID(taskChild.Name)
				if _, exists := entities[invokedJobID]; !exists {
					entities[invokedJobID] = &Entity{
						ID:    invokedJobID,
						Label: taskChild.Name,
						Type:  "job",
					}
				}
				relations = append(relations, Relation{From: taskID, To: invokedJobID, Label: "invokes"})
				result.InvokedJob = invokedJobID

				// Process the invoked job's steps and get exit point
				exitID := processSteps(invokedJobID, taskChild.Children)
				if exitID != "" {
					result.ExitID = exitID
				} else {
					result.ExitID = invokedJobID
				}
			}
		}

		return result
	}

	// Check if this is a run command
	if matches := runRegex.FindStringSubmatch(name); len(matches) > 1 {
		execCount := 1
		if len(node.Children) > 0 {
			execCount = countLeafExecs(node)
		}

		stepID := fmt.Sprintf("%s_step_%d", parentID, index)
		label := truncate(matches[1], 30)
		if execCount > 1 {
			label = fmt.Sprintf("%s (%d execs)", label, execCount)
		}

		entities[stepID] = &Entity{
			ID:        stepID,
			Label:     label,
			Type:      "step",
			ExecCount: execCount,
		}

		addPrevLinks(stepID)
		return &StepResult{StepID: stepID, ExitID: stepID}
	}

	// For other nodes (like mkdir), treat as step
	if node.ID != "" || len(node.Children) == 0 {
		execCount := 1
		if len(node.Children) > 0 {
			execCount = countLeafExecs(node)
		}

		stepID := fmt.Sprintf("%s_step_%d", parentID, index)
		label := truncate(name, 30)
		if execCount > 1 {
			label = fmt.Sprintf("%s (%d execs)", label, execCount)
		}

		entities[stepID] = &Entity{
			ID:        stepID,
			Label:     label,
			Type:      "step",
			ExecCount: execCount,
		}

		addPrevLinks(stepID)
		return &StepResult{StepID: stepID, ExitID: stepID}
	}

	// Recurse for nested structures
	exitID := processSteps(parentID, node.Children)
	if exitID != "" {
		return &StepResult{StepID: exitID, ExitID: exitID}
	}
	return nil
}

func countLeafExecs(node *StateNode) int {
	if len(node.Children) == 0 {
		return 1
	}
	count := 0
	for _, child := range node.Children {
		count += countLeafExecs(child)
	}
	return count
}

func outputPlantUML(title string) {
	fmt.Println("@startuml")
	fmt.Printf("title %s\n", title)
	fmt.Println()
	fmt.Println("skinparam componentStyle rectangle")
	fmt.Println("skinparam defaultTextAlignment center")
	fmt.Println("top to bottom direction")
	fmt.Println()

	// Group by type using components
	for _, e := range entities {
		switch e.Type {
		case "pipeline":
			fmt.Printf("component \"%s\" as %s #LightBlue\n", e.Label, e.ID)
		case "job":
			fmt.Printf("component \"%s\" as %s #LightGreen\n", e.Label, e.ID)
		case "task":
			fmt.Printf("component \"%s\" as %s #Khaki\n", e.Label, e.ID)
		case "step":
			fmt.Printf("component \"%s\" as %s #WhiteSmoke\n", e.Label, e.ID)
		}
	}

	fmt.Println()
	fmt.Println("' Relationships")

	for _, r := range relations {
		switch r.Label {
		case "invokes":
			fmt.Printf("%s ..> %s\n", r.From, r.To)
		case "next":
			fmt.Printf("%s --> %s\n", r.From, r.To)
		case "job":
			fmt.Printf("%s --> %s\n", r.From, r.To)
		default:
			fmt.Printf("%s --> %s\n", r.From, r.To)
		}
	}

	fmt.Println()
	fmt.Println("@enduml")
}

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, ",", "_")
	s = strings.ReplaceAll(s, "=", "_")
	// Limit length
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
