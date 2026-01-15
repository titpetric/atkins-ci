package main

import (
	"fmt"
	"os"

	"github.com/titpetric/cli"
)

// Version information injected at build time via ldflags
var (
	Version    = "dev"
	Commit     = "unknown"
	CommitTime = "unknown"
	Branch     = "unknown"
	Modified   = "false"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	app := cli.NewApp("atkins")
	app.AddCommand("run", "Run pipeline", NewCommand)
	app.DefaultCommand = "run"
	return app.Run()
}
