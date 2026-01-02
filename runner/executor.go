package runner

import (
	"fmt"
	"strings"

	"github.com/titpetric/atkins-ci/colors"
	"github.com/titpetric/atkins-ci/model"
)

// Executor runs pipeline jobs and steps
type Executor struct{}

// NewExecutor creates a new executor
func NewExecutor() *Executor {
	return &Executor{}
}

// ExecuteJob runs a single job
func (e *Executor) ExecuteJob(ctx *model.ExecutionContext, jobName string, job *model.Job) error {
	// Merge job variables into context
	if job.Vars != nil {
		for k, v := range job.Vars {
			ctx.Variables[k] = v
		}
	}

	// Merge job environment
	if job.Env != nil {
		for k, v := range job.Env {
			ctx.Env[k] = v
		}
	}

	// Execute steps
	if len(job.Steps) > 0 {
		return e.executeSteps(ctx, job.Steps)
	}

	// Execute legacy cmd/cmds format
	if job.Run != "" {
		return e.executeCommand(ctx, job.Run)
	}

	if job.Cmd != "" {
		return e.executeCommand(ctx, job.Cmd)
	}

	if len(job.Cmds) > 0 {
		for _, cmd := range job.Cmds {
			if err := e.executeCommand(ctx, cmd); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

// executeSteps runs a sequence of steps
func (e *Executor) executeSteps(ctx *model.ExecutionContext, steps []model.Step) error {
	for _, step := range steps {
		// Handle step-level environment variables
		stepCtx := &model.ExecutionContext{
			Variables: ctx.Variables,
			Env:       make(map[string]string),
			Results:   ctx.Results,
		}

		// Copy parent env and add step-specific env
		for k, v := range ctx.Env {
			stepCtx.Env[k] = v
		}
		if step.Env != nil {
			for k, v := range step.Env {
				stepCtx.Env[k] = v
			}
		}

		// Determine which command to run
		var cmd string
		if step.Run != "" {
			cmd = step.Run
		} else if step.Cmd != "" {
			cmd = step.Cmd
		} else if len(step.Cmds) > 0 {
			cmd = strings.Join(step.Cmds, " && ")
		} else {
			continue
		}

		if err := e.executeCommand(stepCtx, cmd); err != nil {
			colors.PrintFail(step.Name, err.Error())
			return err
		}
		colors.PrintPass(step.Name)
	}
	return nil
}

// executeCommand runs a single command with interpolation
func (e *Executor) executeCommand(ctx *model.ExecutionContext, cmd string) error {
	// Interpolate the command
	interpolated, err := InterpolateCommand(cmd, ctx)
	if err != nil {
		return fmt.Errorf("interpolation failed: %w", err)
	}

	// Execute the command via bash
	output, err := ExecuteCommand(interpolated)
	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	if output != "" {
		fmt.Print(output)
	}

	return nil
}
