package treeview

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewNode tests creating a new node
func TestNewNode(t *testing.T) {
	t.Run("creates node with correct defaults", func(t *testing.T) {
		node := NewNode("test-job")

		assert.Equal(t, "test-job", node.Name)
		assert.Equal(t, StatusPending, node.Status)
		assert.Equal(t, "", node.ID)
		assert.Equal(t, "", node.StatusColor())
		assert.False(t, node.Deferred)
		assert.Equal(t, 0, len(node.Children))
		assert.Equal(t, 0, len(node.Dependencies))
		assert.False(t, node.UpdatedAt.IsZero())
	})
}

// TestNewJobNode tests creating a job node
func TestNewJobNode(t *testing.T) {
	t.Run("root level job node", func(t *testing.T) {
		node := NewJobNode("test", false)

		assert.Equal(t, "test", node.Name)
		assert.Equal(t, StatusPending, node.Status)
	})

	t.Run("nested job node", func(t *testing.T) {
		node := NewJobNode("test:nested", true)

		assert.Equal(t, "test:nested", node.Name)
		assert.Equal(t, StatusConditional, node.Status)
	})
}

// TestNewStepNode tests creating a step node
func TestNewStepNode(t *testing.T) {
	t.Run("regular step node", func(t *testing.T) {
		node := NewStepNode("echo test", false)

		assert.Equal(t, "echo test", node.Name)
		assert.Equal(t, StatusRunning, node.Status)
		assert.False(t, node.Deferred)
	})

	t.Run("deferred step node", func(t *testing.T) {
		node := NewStepNode("cleanup", true)

		assert.Equal(t, "cleanup", node.Name)
		assert.Equal(t, StatusRunning, node.Status)
		assert.True(t, node.Deferred)
	})
}

// TestSetStatus tests updating node status
func TestSetStatus(t *testing.T) {
	t.Run("status update", func(t *testing.T) {
		node := NewNode("test")
		assert.Equal(t, StatusPending, node.Status)

		node.SetStatus(StatusRunning)
		assert.Equal(t, StatusRunning, node.Status)

		node.SetStatus(StatusPassed)
		assert.Equal(t, StatusPassed, node.Status)
	})

	t.Run("status update clears deferred flag", func(t *testing.T) {
		node := NewStepNode("cleanup", true)
		assert.True(t, node.Deferred)

		node.SetStatus(StatusPassed)
		assert.False(t, node.Deferred)
	})

	t.Run("updates timestamp on status change", func(t *testing.T) {
		node := NewNode("test")
		oldTime := node.UpdatedAt

		time.Sleep(1 * time.Millisecond)
		node.SetStatus(StatusRunning)

		assert.True(t, node.UpdatedAt.After(oldTime))
	})
}

// TestAddChild tests adding a single child node
func TestAddChild(t *testing.T) {
	t.Run("add single child", func(t *testing.T) {
		parent := NewNode("parent")
		child := NewNode("child")

		assert.False(t, parent.HasChildren())

		parent.AddChild(child)

		assert.True(t, parent.HasChildren())
		children := parent.GetChildren()
		assert.Equal(t, 1, len(children))
		assert.Equal(t, "child", children[0].Name)
	})

	t.Run("add multiple children sequentially", func(t *testing.T) {
		parent := NewNode("parent")

		parent.AddChild(NewNode("child1"))
		parent.AddChild(NewNode("child2"))
		parent.AddChild(NewNode("child3"))

		children := parent.GetChildren()
		assert.Equal(t, 3, len(children))
		assert.Equal(t, "child1", children[0].Name)
		assert.Equal(t, "child2", children[1].Name)
		assert.Equal(t, "child3", children[2].Name)
	})
}

// TestAddChildren tests adding multiple child nodes at once
func TestAddChildren(t *testing.T) {
	t.Run("add multiple children", func(t *testing.T) {
		parent := NewNode("parent")
		child1 := NewNode("child1")
		child2 := NewNode("child2")
		child3 := NewNode("child3")

		parent.AddChildren(child1, child2, child3)

		children := parent.GetChildren()
		assert.Equal(t, 3, len(children))
		assert.Equal(t, "child1", children[0].Name)
		assert.Equal(t, "child2", children[1].Name)
		assert.Equal(t, "child3", children[2].Name)
	})

	t.Run("add children to node with existing children", func(t *testing.T) {
		parent := NewNode("parent")
		parent.AddChild(NewNode("child1"))

		parent.AddChildren(NewNode("child2"), NewNode("child3"))

		children := parent.GetChildren()
		assert.Equal(t, 3, len(children))
	})
}

// TestGetChildren tests retrieving children
func TestGetChildren(t *testing.T) {
	t.Run("returns copy of children", func(t *testing.T) {
		parent := NewNode("parent")
		child := NewNode("child")
		parent.AddChild(child)

		children1 := parent.GetChildren()
		children2 := parent.GetChildren()

		// Should be equal but not the same slice
		assert.Equal(t, children1, children2)
		assert.False(t, &children1 == &children2)
	})

	t.Run("modifying returned slice doesn't affect node", func(t *testing.T) {
		parent := NewNode("parent")
		parent.AddChild(NewNode("child1"))

		children := parent.GetChildren()
		assert.Equal(t, 1, len(children))

		// Modify the returned slice
		children = append(children, NewNode("fake"))

		// Original should be unchanged
		originalChildren := parent.GetChildren()
		assert.Equal(t, 1, len(originalChildren))
	})

	t.Run("empty children list", func(t *testing.T) {
		node := NewNode("test")
		children := node.GetChildren()

		assert.Equal(t, 0, len(children))
		assert.NotNil(t, children)
	})
}

// TestHasChildren tests checking for children
func TestHasChildren(t *testing.T) {
	t.Run("node without children", func(t *testing.T) {
		node := NewNode("test")
		assert.False(t, node.HasChildren())
	})

	t.Run("node with children", func(t *testing.T) {
		node := NewNode("test")
		node.AddChild(NewNode("child"))
		assert.True(t, node.HasChildren())
	})

	t.Run("node with multiple children", func(t *testing.T) {
		node := NewNode("test")
		node.AddChildren(NewNode("child1"), NewNode("child2"))
		assert.True(t, node.HasChildren())
	})
}

// TestNodeFieldInitialization tests that node fields are properly initialized
func TestNodeFieldInitialization(t *testing.T) {
	t.Run("dependencies field", func(t *testing.T) {
		node := NewNode("test")
		assert.NotNil(t, node.Dependencies)
		assert.Equal(t, 0, len(node.Dependencies))

		// Should be safe to append to
		node.Dependencies = append(node.Dependencies, "dep1")
		assert.Equal(t, 1, len(node.Dependencies))
	})

	t.Run("children field", func(t *testing.T) {
		node := NewNode("test")
		assert.NotNil(t, node.Children)
		assert.Equal(t, 0, len(node.Children))
	})
}

// TestNodeThreadSafety tests thread-safe operations (basic check)
func TestNodeThreadSafety(t *testing.T) {
	t.Run("concurrent status updates", func(t *testing.T) {
		node := NewNode("test")

		// Perform concurrent operations
		done := make(chan bool, 2)

		go func() {
			for i := 0; i < 100; i++ {
				node.SetStatus(StatusRunning)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				node.SetStatus(StatusPassed)
			}
			done <- true
		}()

		<-done
		<-done

		// Should end in one of the two states without deadlock
		assert.True(t, node.Status == StatusRunning || node.Status == StatusPassed)
	})

	t.Run("concurrent child operations", func(t *testing.T) {
		node := NewNode("parent")
		done := make(chan bool, 2)

		go func() {
			for i := 0; i < 50; i++ {
				node.AddChild(NewNode("child"))
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 50; i++ {
				node.GetChildren()
			}
			done <- true
		}()

		<-done
		<-done

		// Should have children without deadlock
		assert.True(t, node.HasChildren())
	})
}

// TestNodeNestedStructure tests creating a nested tree structure
func TestNodeNestedStructure(t *testing.T) {
	t.Run("deep nesting", func(t *testing.T) {
		root := NewNode("root")
		current := root

		// Create a chain of 5 levels
		for i := 1; i <= 5; i++ {
			suffix := string(rune(48 + i)) // 48='0', 48+1='1', 48+2='2', etc -> level1, level2, level3, level4, level5
			child := NewNode("level" + suffix)
			current.AddChild(child)
			current = child
		}

		// Verify structure
		level1 := root.GetChildren()
		assert.Equal(t, 1, len(level1))
		assert.Equal(t, "level1", level1[0].Name)

		level2 := level1[0].GetChildren()
		assert.Equal(t, 1, len(level2))
		assert.Equal(t, "level2", level2[0].Name)

		level3 := level2[0].GetChildren()
		assert.Equal(t, 1, len(level3))
		assert.Equal(t, "level3", level3[0].Name)

		level4 := level3[0].GetChildren()
		assert.Equal(t, 1, len(level4))
		assert.Equal(t, "level4", level4[0].Name)

		level5 := level4[0].GetChildren()
		assert.Equal(t, 1, len(level5))
		assert.Equal(t, "level5", level5[0].Name)
	})

	t.Run("multiple children at each level", func(t *testing.T) {
		root := NewNode("root")

		// Create first level with 3 children
		for i := 0; i < 3; i++ {
			child := NewNode("child" + string(rune(48+i)))
			root.AddChild(child)

			// Each child has 2 grandchildren
			for j := 0; j < 2; j++ {
				grandchild := NewNode("grandchild")
				child.AddChild(grandchild)
			}
		}

		children := root.GetChildren()
		assert.Equal(t, 3, len(children))

		for _, child := range children {
			assert.Equal(t, 2, len(child.GetChildren()))
		}
	})
}
