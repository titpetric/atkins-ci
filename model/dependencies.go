package model

import yaml "gopkg.in/yaml.v3"

// Dependencies represents job dependencies.
type Dependencies []string

// UnmarshalYAML implements custom unmarshalling for `depends_on`,
// taking a string value, or a slice of strings.
func (s *Dependencies) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = Dependencies([]string{node.Value})
		return nil
	}

	var deps []string
	if err := node.Decode(&deps); err != nil {
		return err
	}
	*s = Dependencies(deps)
	return nil
}
