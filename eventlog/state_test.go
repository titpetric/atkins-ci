package eventlog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/atkins/treeview"
)

func TestNodeToStateNode(t *testing.T) {
	node := treeview.NewNode("test-node")
	node.ID = "jobs.test.steps.0"
	node.SetStatus(treeview.StatusPassed)
	node.SetStartOffset(0.5)
	node.SetDuration(0.1)
	node.SetIf("${{ always }}")

	state := NodeToStateNode(node)

	assert.Equal(t, "test-node", state.Name)
	assert.Equal(t, "jobs.test.steps.0", state.ID)
	assert.Equal(t, "passed", state.Status)
	assert.Equal(t, ResultPass, state.Result)
	assert.Equal(t, "${{ always }}", state.If)
	assert.Equal(t, 0.5, state.Start)
	assert.Equal(t, 0.1, state.Duration)
}

func TestNodeToStateNode_Nil(t *testing.T) {
	state := NodeToStateNode(nil)
	assert.Nil(t, state)
}

func TestNodeToStateNode_WithChildren(t *testing.T) {
	root := treeview.NewNode("root")
	child1 := treeview.NewNode("child1")
	child1.SetStatus(treeview.StatusPassed)
	child2 := treeview.NewNode("child2")
	child2.SetStatus(treeview.StatusFailed)

	root.AddChild(child1)
	root.AddChild(child2)

	state := NodeToStateNode(root)

	assert.Len(t, state.Children, 2)
	assert.Equal(t, "child1", state.Children[0].Name)
	assert.Equal(t, ResultPass, state.Children[0].Result)
	assert.Equal(t, "child2", state.Children[1].Name)
	assert.Equal(t, ResultFail, state.Children[1].Result)
}

func TestCountSteps(t *testing.T) {
	root := &StateNode{
		Name:   "root",
		Result: ResultPass,
		Children: []*StateNode{
			{Name: "job1", Children: []*StateNode{
				{Name: "step1", Result: ResultPass},
				{Name: "step2", Result: ResultPass},
			}},
			{Name: "job2", Children: []*StateNode{
				{Name: "step3", Result: ResultFail},
				{Name: "step4", Result: ResultSkipped},
			}},
		},
	}

	total, passed, failed, skipped := CountSteps(root)

	assert.Equal(t, 4, total)
	assert.Equal(t, 2, passed)
	assert.Equal(t, 1, failed)
	assert.Equal(t, 1, skipped)
}

func TestCountSteps_Nil(t *testing.T) {
	total, passed, failed, skipped := CountSteps(nil)

	assert.Equal(t, 0, total)
	assert.Equal(t, 0, passed)
	assert.Equal(t, 0, failed)
	assert.Equal(t, 0, skipped)
}

func TestStatusToResult(t *testing.T) {
	tests := []struct {
		status   treeview.Status
		expected Result
	}{
		{treeview.StatusPassed, ResultPass},
		{treeview.StatusFailed, ResultFail},
		{treeview.StatusSkipped, ResultSkipped},
		{treeview.StatusPending, ""},
		{treeview.StatusRunning, ""},
		{treeview.StatusConditional, ""},
	}

	for _, tt := range tests {
		t.Run(tt.status.Label(), func(t *testing.T) {
			assert.Equal(t, tt.expected, statusToResult(tt.status))
		})
	}
}

func TestTreeNodeToStateNode(t *testing.T) {
	node := treeview.NewNode("test")
	node.SetStatus(treeview.StatusPassed)
	treeNode := &treeview.TreeNode{Node: node}

	state := TreeNodeToStateNode(treeNode)

	assert.Equal(t, "test", state.Name)
	assert.Equal(t, "passed", state.Status)
}

func TestTreeNodeToStateNode_Nil(t *testing.T) {
	state := TreeNodeToStateNode(nil)
	assert.Nil(t, state)
}

func TestCalculateDuration(t *testing.T) {
	state := &StateNode{
		Duration: 1.5,
	}

	assert.Equal(t, 1.5, CalculateDuration(state))
	assert.Equal(t, 0.0, CalculateDuration(nil))
}

func TestNodeToStateNode_Times(t *testing.T) {
	node := treeview.NewNode("test")

	// Verify times are set
	assert.False(t, node.CreatedAt.IsZero())
	assert.False(t, node.UpdatedAt.IsZero())

	state := NodeToStateNode(node)
	assert.False(t, state.CreatedAt.IsZero())
}

func TestNodeToStateNode_AllStatuses(t *testing.T) {
	tests := []struct {
		status        treeview.Status
		expectedLabel string
	}{
		{treeview.StatusPending, "pending"},
		{treeview.StatusRunning, "running"},
		{treeview.StatusPassed, "passed"},
		{treeview.StatusFailed, "failed"},
		{treeview.StatusSkipped, "skipped"},
		{treeview.StatusConditional, "conditional"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedLabel, func(t *testing.T) {
			node := treeview.NewNode("test")
			node.SetStatus(tt.status)

			state := NodeToStateNode(node)
			assert.Equal(t, tt.expectedLabel, state.Status)
		})
	}
}
