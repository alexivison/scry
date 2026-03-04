package main

import (
	"fmt"
	"os"

	"github.com/alexivison/scry/internal/config"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	os.Exit(runWith(os.Args[1:]))
}

func runWith(args []string) int {
	cfg, err := config.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 2
	}

	// TODO(T6): bootstrap app and launch TUI using cfg.
	_ = cfg

	fmt.Printf("scry %s (%s)\n", version, commit)
	return 0
}
