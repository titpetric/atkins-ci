package runner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/atkins/model"
	"github.com/titpetric/atkins/runner"
)

func TestVariableEvaluation_DependencyOrdering(t *testing.T) {
	ctx := &runner.ExecutionContext{
		Variables: make(map[string]any),
		Env: map[string]string{
			"CI": "true",
		},
	}

	decl := &model.Decl{
		Vars: map[string]any{
			// Plain strings
			"plain_string": "hello world",
			"plain_number": 42,

			// Bash command evaluation
			"bash_result":    "$(echo computed)",
			"bash_with_args": "$(printf '%s-%s' one two)",

			// Simple interpolation referencing plain values
			"interpolated_simple": "${{ plain_string }}",

			// Chain dependency
			"chain_level_2": "${{ interpolated_simple }}-suffix",
			"chain_level_3": "prefix-${{ chain_level_2 }}",

			// Mixed: bash + interpolation
			"mixed_bash_interp": "$(echo ${{ plain_string }})",

			// Multiple dependencies in one value
			"multi_dep": "${{ plain_string }} and ${{ plain_number }}",

			// Deep chain stress test
			"base_value":  "base",
			"derived_a":   "${{ base_value }}_a",
			"derived_b":   "${{ derived_a }}_b",
			"derived_c":   "${{ derived_b }}_c",
			"final_chain": "${{ derived_c }}_final",

			// Env var access via printenv
			"ci_value":    "$(printenv CI)",
			"build_label": "${{ plain_string }}-ci=${{ ci_value }}",

			// Env var name via interpolation
			"env_name": "CI",
			"read_env": "$(printenv ${{ env_name }})",
		},
	}

	err := runner.MergeVariables(decl, ctx)
	require.NoError(t, err)

	expected := map[string]any{
		"plain_string":        "hello world",
		"plain_number":        42,
		"bash_result":         "computed",
		"bash_with_args":      "one-two",
		"interpolated_simple": "hello world",
		"chain_level_2":       "hello world-suffix",
		"chain_level_3":       "prefix-hello world-suffix",
		"mixed_bash_interp":   "hello world",
		"multi_dep":           "hello world and 42",
		"base_value":          "base",
		"derived_a":           "base_a",
		"derived_b":           "base_a_b",
		"derived_c":           "base_a_b_c",
		"final_chain":         "base_a_b_c_final",
		"ci_value":            "true",
		"build_label":         "hello world-ci=true",
		"env_name":            "CI",
		"read_env":            "true",
	}

	for k, expectedVal := range expected {
		assert.Equal(t, expectedVal, ctx.Variables[k], "variable %q mismatch", k)
	}
}

func TestVariableEvaluation_CyclicDependency(t *testing.T) {
	ctx := &runner.ExecutionContext{
		Variables: make(map[string]any),
		Env:       make(map[string]string),
	}

	decl := &model.Decl{
		Vars: map[string]any{
			"a": "${{ b }}",
			"b": "${{ a }}",
		},
	}

	err := runner.MergeVariables(decl, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}
