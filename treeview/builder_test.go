package treeview

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/atkins-ci/model"
)

// TestBuildFromPipeline_SingleJob tests building a pipeline with a single job
func TestBuildFromPipeline_SingleJob(t *testing.T) {
	t.Run("single job with steps", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Jobs: map[string]*model.Job{
				"test": {
					Desc: "Test job",
					Steps: []*model.Step{
						{Run: "go test ./...", Name: "run tests"},
					},
				},
			},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, "test-pipeline", node.Name)
		assert.True(t, node.HasChildren())

		children := node.GetChildren()
		assert.Equal(t, 1, len(children))
		assert.Equal(t, "test", children[0].Name)
	})
}

// TestBuildFromPipeline_DepthSorting tests that jobs are sorted by depth then name
func TestBuildFromPipeline_DepthSorting(t *testing.T) {
	t.Run("depth-based ordering", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Jobs: map[string]*model.Job{
				"test":             {Desc: "Test"},
				"test:run":         {Desc: "Test run"},
				"test:run:subtask": {Desc: "Test run subtask"},
				"build":            {Desc: "Build"},
				"build:run":        {Desc: "Build run"},
				"docker:setup":     {Desc: "Docker setup"},
			},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)

		children := node.GetChildren()
		assert.Equal(t, 6, len(children))

		// Expected order: depth 0 (build, test), depth 1 (build:run, docker:setup, test:run), depth 2 (test:run:subtask)
		expectedOrder := []string{
			"build",
			"test",
			"build:run",
			"docker:setup",
			"test:run",
			"test:run:subtask",
		}

		for i, expected := range expectedOrder {
			assert.Equal(t, expected, children[i].Name, "job order mismatch at index %d", i)
		}
	})

	t.Run("nested depth sorting consistency", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Jobs: map[string]*model.Job{
				"zebra":     {Desc: "Zebra"},
				"zebra:a":   {Desc: "Zebra A"},
				"apple":     {Desc: "Apple"},
				"apple:b":   {Desc: "Apple B"},
				"apple:b:c": {Desc: "Apple B C"},
			},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)

		children := node.GetChildren()
		assert.Equal(t, 5, len(children))

		// Expected: apple (depth 0), zebra (depth 0), apple:b (depth 1), zebra:a (depth 1), apple:b:c (depth 2)
		expectedOrder := []string{
			"apple",
			"zebra",
			"apple:b",
			"zebra:a",
			"apple:b:c",
		}

		for i, expected := range expectedOrder {
			assert.Equal(t, expected, children[i].Name, "job order mismatch at index %d", i)
		}
	})
}

// TestBuildFromPipeline_WithTasks tests building a pipeline using tasks instead of jobs
func TestBuildFromPipeline_WithTasks(t *testing.T) {
	t.Run("pipeline with tasks", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Tasks: map[string]*model.Job{
				"build": {Desc: "Build task"},
				"test":  {Desc: "Test task"},
			},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)

		children := node.GetChildren()
		assert.Equal(t, 2, len(children))
	})
}

// TestBuildFromPipeline_EmptyPipeline tests building an empty pipeline
func TestBuildFromPipeline_EmptyPipeline(t *testing.T) {
	t.Run("empty pipeline", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name:  "empty-pipeline",
			Jobs:  map[string]*model.Job{},
			Tasks: map[string]*model.Job{},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)
		assert.NotNil(t, node)
		assert.False(t, node.HasChildren())
	})
}

// TestBuildFromPipeline_WithDependencies tests that dependencies are preserved
func TestBuildFromPipeline_WithDependencies(t *testing.T) {
	t.Run("jobs with dependencies", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Jobs: map[string]*model.Job{
				"build": {Desc: "Build"},
				"test": {
					Desc:      "Test",
					DependsOn: model.Dependencies([]string{"build"}),
				},
			},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)

		children := node.GetChildren()
		assert.Equal(t, 2, len(children))

		// Check that test job has the dependency
		testNode := children[1] // test should be at index 1 after sorting
		assert.Equal(t, "test", testNode.Name)
		assert.Equal(t, []string{"build"}, testNode.Dependencies)
	})
}

// TestAddJob_WithSteps tests adding a job with steps
func TestAddJob_WithSteps(t *testing.T) {
	t.Run("job with multiple steps", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc: "Test job",
			Steps: []*model.Step{
				{Run: "echo 1", Name: "step 1"},
				{Run: "echo 2", Name: "step 2"},
				{Run: "echo 3", Name: "step 3"},
			},
		}

		treeNode := builder.AddJob(job, []string{}, "test")
		assert.NotNil(t, treeNode)
		assert.Equal(t, "test", treeNode.Node.Name)

		children := treeNode.Node.GetChildren()
		assert.Equal(t, 3, len(children))

		// Verify step names
		assert.Equal(t, "run: echo 1", children[0].Name)
		assert.Equal(t, "run: echo 2", children[1].Name)
		assert.Equal(t, "run: echo 3", children[2].Name)
	})

	t.Run("job without steps", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc: "Test job",
		}

		treeNode := builder.AddJob(job, []string{}, "test")
		assert.NotNil(t, treeNode)
		assert.False(t, treeNode.Node.HasChildren())
	})

	t.Run("job with task invocation steps", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc: "Test job with task steps",
			Steps: []*model.Step{
				{Task: "build", Name: "build step"},
				{Task: "test", Name: "test step"},
				{Run: "echo cleanup", Name: "cleanup step"},
			},
		}

		treeNode := builder.AddJob(job, []string{}, "test")
		assert.NotNil(t, treeNode)

		children := treeNode.Node.GetChildren()
		assert.Equal(t, 3, len(children))

		// Verify that task steps are properly converted to node names
		assert.Equal(t, "task: build", children[0].Name)
		assert.Equal(t, "task: test", children[1].Name)
		assert.Equal(t, "run: echo cleanup", children[2].Name)
	})

	t.Run("job with summarized step", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc: "Test job with summarized steps",
			Steps: []*model.Step{
				{Run: "echo iteration1", Name: "iter 1"},
				{Run: "echo iteration2", Name: "iter 2", Summarize: true},
				{Run: "echo iteration3", Name: "iter 3"},
			},
		}

		treeNode := builder.AddJob(job, []string{}, "test")
		assert.NotNil(t, treeNode)

		children := treeNode.Node.GetChildren()
		assert.Equal(t, 3, len(children))

		// Verify that Summarize flag is copied to the step node
		assert.False(t, children[0].Summarize)
		assert.True(t, children[1].Summarize)
		assert.False(t, children[2].Summarize)
	})

	t.Run("job with deferred and summarized step", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc: "Test job with deferred and summarized step",
			Steps: []*model.Step{
				{Run: "echo main", Name: "main step"},
				{Run: "echo cleanup", Name: "cleanup", Deferred: true, Summarize: true},
			},
		}

		treeNode := builder.AddJob(job, []string{}, "test")
		children := treeNode.Node.GetChildren()
		assert.Equal(t, 2, len(children))

		// Verify both flags are copied
		cleanupStep := children[1]
		assert.True(t, cleanupStep.Deferred)
		assert.True(t, cleanupStep.Summarize)
	})
}

// TestAddJobWithoutSteps tests the AddJobWithoutSteps method
func TestAddJobWithoutSteps(t *testing.T) {
	t.Run("adding job without steps", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")

		treeNode := builder.AddJobWithoutSteps([]string{"dep1", "dep2"}, "test", false)
		assert.NotNil(t, treeNode)
		assert.Equal(t, "test", treeNode.Node.Name)
		assert.Equal(t, []string{"dep1", "dep2"}, treeNode.Node.Dependencies)
		assert.False(t, treeNode.Node.HasChildren())
	})

	t.Run("nested job flag", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")

		treeNode := builder.AddJobWithoutSteps([]string{}, "test:nested", true)
		assert.NotNil(t, treeNode)
		assert.Equal(t, StatusConditional, treeNode.Node.Status)
	})
}

// TestBuildFromPipeline_ConsistentOrdering tests that ordering is consistent across multiple builds
func TestBuildFromPipeline_ConsistentOrdering(t *testing.T) {
	t.Run("consistent ordering across builds", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Jobs: map[string]*model.Job{
				"zebra":    {Desc: "Zebra"},
				"apple":    {Desc: "Apple"},
				"banana":   {Desc: "Banana"},
				"test:run": {Desc: "Test run"},
				"test":     {Desc: "Test"},
			},
		}

		// Build multiple times
		node1, _ := BuildFromPipeline(pipeline, mockResolveDeps)
		node2, _ := BuildFromPipeline(pipeline, mockResolveDeps)
		node3, _ := BuildFromPipeline(pipeline, mockResolveDeps)

		children1 := node1.GetChildren()
		children2 := node2.GetChildren()
		children3 := node3.GetChildren()

		// All should have the same order
		for i := range children1 {
			assert.Equal(t, children1[i].Name, children2[i].Name)
			assert.Equal(t, children1[i].Name, children3[i].Name)
		}
	})
}

// TestAddJob_WithSummaryFlag tests that the Summarize flag is copied from the job model
func TestAddJob_WithSummaryFlag(t *testing.T) {
	t.Run("job with summarize enabled", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc:      "Test job with summary",
			Summarize: true,
			Steps: []*model.Step{
				{Run: "echo 1", Name: "step 1"},
				{Run: "echo 2", Name: "step 2"},
			},
		}

		treeNode := builder.AddJob(job, []string{}, "test-summary")
		assert.NotNil(t, treeNode)
		assert.Equal(t, "test-summary", treeNode.Node.Name)
		assert.True(t, treeNode.Node.Summarize)

		children := treeNode.Node.GetChildren()
		assert.Equal(t, 2, len(children))
	})

	t.Run("job with summarize disabled", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc:      "Test job without summary",
			Summarize: false,
			Steps: []*model.Step{
				{Run: "echo 1", Name: "step 1"},
			},
		}

		treeNode := builder.AddJob(job, []string{}, "test-no-summary")
		assert.NotNil(t, treeNode)
		assert.False(t, treeNode.Node.Summarize)
	})
}

// TestAddJobWithSummary tests the AddJobWithSummary helper method
func TestAddJobWithSummary(t *testing.T) {
	t.Run("adding job with summary enabled", func(t *testing.T) {
		builder := NewBuilder("test-pipeline")
		job := &model.Job{
			Desc: "Test job",
			Steps: []*model.Step{
				{Run: "echo test", Name: "test step"},
			},
		}

		treeNode := builder.AddJobWithSummary(job, []string{}, "summary-job")
		assert.NotNil(t, treeNode)
		assert.Equal(t, "summary-job", treeNode.Node.Name)
		assert.True(t, treeNode.Node.Summarize)

		children := treeNode.Node.GetChildren()
		assert.Equal(t, 1, len(children))
	})
}

// TestBuildFromPipeline_WithSummarize tests building a pipeline with summarized jobs
func TestBuildFromPipeline_WithSummarize(t *testing.T) {
	t.Run("pipeline with summarized job", func(t *testing.T) {
		pipeline := &model.Pipeline{
			Name: "test-pipeline",
			Jobs: map[string]*model.Job{
				"test": {
					Desc:      "Test job",
					Summarize: true,
					Steps: []*model.Step{
						{Run: "go test ./...", Name: "run tests"},
						{Run: "go coverage", Name: "coverage"},
					},
				},
			},
		}

		node, err := BuildFromPipeline(pipeline, mockResolveDeps)
		assert.NoError(t, err)
		assert.NotNil(t, node)

		children := node.GetChildren()
		assert.Equal(t, 1, len(children))
		assert.Equal(t, "test", children[0].Name)
		assert.True(t, children[0].Summarize)
	})
}

// mockResolveDeps is a mock function for resolving dependencies
func mockResolveDeps(jobs map[string]*model.Job, startingJob string) ([]string, error) {
	result := make([]string, 0, len(jobs))
	for name := range jobs {
		result = append(result, name)
	}
	return result, nil
}
