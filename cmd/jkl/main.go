package main

import (
	"fmt"
	"jkl"
	"os"
)

func main() {
	j, err := jkl.New(jkl.WithInstallsDir("~/.jkl/installs"), jkl.WithShimsDir("~/.jkl/bin"))

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	err = j.RunCLI(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
