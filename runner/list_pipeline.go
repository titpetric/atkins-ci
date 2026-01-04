package runner

import (
	"github.com/titpetric/atkins-ci/model"
	"github.com/titpetric/atkins-ci/treeview"
)

// ListPipeline displays a pipeline's job tree with dependencies
func ListPipeline(pipeline *model.Pipeline) error {
	allJobs := pipeline.Jobs
	if len(allJobs) == 0 {
		allJobs = pipeline.Tasks
	}

	node, err := treeview.BuildFromPipeline(pipeline, ResolveJobDependencies)
	if err != nil {
		return err
	}

	display := treeview.NewDisplay()
	display.RenderStatic(node)
	return nil
}
