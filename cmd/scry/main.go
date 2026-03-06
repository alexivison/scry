package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alexivison/scry/internal/app"
	"github.com/alexivison/scry/internal/config"
	flag "github.com/spf13/pflag"
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
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 2
	}

	return app.Run(cfg)
}
