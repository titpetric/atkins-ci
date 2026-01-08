package model

import (
	"fmt"

	yaml "gopkg.in/yaml.v3"
)

// ensureDeclInitialized ensures that the Decl is initialized and that vars/include
// are properly decoded from the YAML node. This handles the case where embedded *Decl
// pointers might not be properly unmarshalled.
//
// This helper should be called in UnmarshalYAML for any type that embeds *Decl.
func ensureDeclInitialized(decl **Decl, node *yaml.Node) error {
	if *decl == nil {
		*decl = &Decl{}
	}

	// If vars/include/env weren't decoded into the embedded Decl (which can happen with embedded pointers),
	// try decoding them explicitly from the YAML node
	if ((*decl).Vars == nil || (*decl).Include == nil || (*decl).Env == nil) && node != nil && node.Content != nil && len(node.Content) > 0 {
		for i := 0; i < len(node.Content)-1; i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			if keyNode.Value == "vars" && (*decl).Vars == nil {
				if err := valueNode.Decode(&(*decl).Vars); err != nil {
					return fmt.Errorf("failed to decode vars: %w", err)
				}
			}
			if keyNode.Value == "include" && (*decl).Include == nil {
				if err := valueNode.Decode(&(*decl).Include); err != nil {
					return fmt.Errorf("failed to decode include: %w", err)
				}
			}
			if keyNode.Value == "env" && (*decl).Env == nil {
				if err := valueNode.Decode(&(*decl).Env); err != nil {
					return fmt.Errorf("failed to decode env: %w", err)
				}
			}
		}
	}

	return nil
}
