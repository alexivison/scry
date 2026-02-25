package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
)

func main() {
	fmt.Printf("scry %s (%s)\n", version, commit)
}
