package main_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"
)

type Result struct {
	Name        string        `yaml:"name,omitempty"`
	PackageName string        `yaml:"packageName"`
	ImportPath  string        `yaml:"importPath"`
	Packages    []string      `yaml:"packages"`
	Types       []StructType  `yaml:"types"`
	Funcs       []PackageFunc `yaml:"funcs"`
	Totals      TotalOutput   `yaml:"total"`
}

type StructType struct {
	Name        string        `yaml:"name"`
	PackageName string        `yaml:"packageName"`
	Funcs       []FuncDetails `yaml:"funcs"`
}

type FuncDetails struct {
	Name     string `yaml:"name"`
	Coverage int    `yaml:"coverage"`
}

type PackageFunc struct {
	Name        string `yaml:"name"`
	PackageName string `yaml:"packageName"`
	Coverage    int    `yaml:"coverage"`
}

type TotalOutput struct {
	Coverage struct {
		Funcs   int `yaml:"funcs"`
		Structs int `yaml:"structs"`
		Total   int `yaml:"total"`
	} `yaml:"coverage"`
}

func TestCoverageAggregate(t *testing.T) {
	testCoverageAggregate(t)
}

func testCoverageAggregate(t *testing.T) {
	dir := os.DirFS("./coverage")

	// Map of package -> TestFunctionName -> *Result
	packageResults := map[string]map[string]*Result{}

	// Reverse lookup of package -> symbol -> *Result (test)
	reverseLookup := map[string]map[string][]*Result{}

	fill := func(r *Result, name string) {
		packageName := name
		packageName = filepath.Dir(packageName)
		packageName = strings.ReplaceAll(packageName, "_", "/")
		if strings.HasSuffix(packageName, ".test") {
			packageName = packageName[:len(packageName)-5]
		}
		r.PackageName = packageName

		if _, ok := packageResults[packageName]; !ok {
			packageResults[packageName] = make(map[string]*Result)
		}
		packageResults[packageName][r.Name] = r

		for _, st := range r.Types {
			packageName := r.PackageName
			if _, ok := reverseLookup[packageName]; !ok {
				reverseLookup[packageName] = make(map[string][]*Result)
			}
			for _, fn := range st.Funcs {
				symbol := st.Name + "." + fn.Name

				reverseLookup[packageName][symbol] = append(reverseLookup[packageName][symbol], r)
			}
		}
		for _, fn := range r.Funcs {
			packageName := r.PackageName
			if _, ok := reverseLookup[packageName]; !ok {
				reverseLookup[packageName] = make(map[string][]*Result)
			}

			reverseLookup[packageName][fn.Name] = append(reverseLookup[packageName][fn.Name], r)
		}
	}

	files := []string{}

	err := fs.WalkDir(dir, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if !strings.HasSuffix(d.Name(), ".yml") {
			return nil
		}

		files = append(files, name)
		return nil
	})
	assert.NoError(t, err)

	for _, name := range files {
		data, err := fs.ReadFile(dir, name)
		assert.NoError(t, err, "error reading file %s", name)

		r := &Result{}
		assert.NoError(t, yaml.Unmarshal(data, r), "error unmarshalling file %s", name)

		fill(r, name)
	}

	for pkgName, syms := range reverseLookup {
		fmt.Println("-", pkgName)
		for symbol, tests := range syms {
			fmt.Printf("  - %s (%d tests)\n", symbol, len(tests))
		}
	}
}
