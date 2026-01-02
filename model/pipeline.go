package model

// Pipeline represents the root structure of an atkins.yml file
type Pipeline struct {
	Name  string          `yaml:"name"`
	Jobs  map[string]*Job `yaml:"jobs"`
	Tasks map[string]*Job `yaml:"tasks"`
}

// Job represents a job/task in the pipeline
type Job struct {
	Desc      string                 `yaml:"desc"`
	RunsOn    string                 `yaml:"runs_on"`
	Container string                 `yaml:"container"`
	If        string                 `yaml:"if"`
	Cmd       string                 `yaml:"cmd"`
	Cmds      []string               `yaml:"cmds"`
	Run       string                 `yaml:"run"`
	Steps     []Step                 `yaml:"steps"`
	Services  map[string]*Service    `yaml:"services"`
	Vars      map[string]interface{} `yaml:"vars"`
	Env       map[string]string      `yaml:"env"`
	Matrix    map[string]interface{} `yaml:"matrix"`
}

// Step represents a step within a job
type Step struct {
	Name string                 `yaml:"name"`
	Desc string                 `yaml:"desc"`
	Run  string                 `yaml:"run"`
	Cmd  string                 `yaml:"cmd"`
	Cmds []string               `yaml:"cmds"`
	If   string                 `yaml:"if"`
	Env  map[string]string      `yaml:"env"`
	Uses string                 `yaml:"uses"`
	With map[string]interface{} `yaml:"with"`
}

// Service represents a service (e.g., Docker container) used in a job
type Service struct {
	Image    string            `yaml:"image"`
	Pull     string            `yaml:"pull"`
	Options  string            `yaml:"options"`
	Ports    []string          `yaml:"ports"`
	Env      map[string]string `yaml:"env"`
	Networks []string          `yaml:"networks"`
}

// ExecutionContext holds runtime state during pipeline execution
type ExecutionContext struct {
	Variables map[string]interface{}
	Env       map[string]string
	Results   map[string]interface{}
}
