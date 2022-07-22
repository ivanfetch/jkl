package main

import (
	"fmt"
	"os"

	"github.com/ivanfetch/jkl"
)

func main() {
	err := jkl.RunCLI(os.Args, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
