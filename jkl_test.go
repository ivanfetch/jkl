package jkl_test

import (
	"os"

	"github.com/ivanfetch/jkl"
)

func init() {
	// Enable debugging for all tests, via the same environment variable the jkl
	// binary uses.
	if os.Getenv("JKL_DEBUG") != "" {
		jkl.EnableDebugOutput()
	}
}
