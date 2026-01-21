package runner

import (
	"fmt"
	"strings"
)

// extractVariableDependencies extracts variable names referenced via ${{ varName }} in a string.
// Only returns dependencies that exist in the vars map.
func extractVariableDependencies(s string, vars map[string]any) []string {
	matches := interpolationRegex.FindAllStringSubmatch(s, -1)
	var deps []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			varName := strings.TrimSpace(match[1])
			if _, exists := vars[varName]; exists && !seen[varName] {
				deps = append(deps, varName)
				seen[varName] = true
			}
		}
	}
	return deps
}

// topologicalSort performs a topological sort on the dependency graph.
// Returns an error if a cycle is detected.
func topologicalSort(deps map[string][]string) ([]string, error) {
	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=visited
	var order []string

	var visit func(node string) error
	visit = func(node string) error {
		if visited[node] == 1 {
			return fmt.Errorf("cycle detected involving variable %q", node)
		}
		if visited[node] == 2 {
			return nil
		}
		visited[node] = 1
		for _, dep := range deps[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visited[node] = 2
		order = append(order, node)
		return nil
	}

	for node := range deps {
		if visited[node] == 0 {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}
	return order, nil
}
