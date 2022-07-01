package main

import (
	"fmt"
	"jkl"
	"os"
)

func main() {
	err := jkl.RunCLI(os.Args, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
