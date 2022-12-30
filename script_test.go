//go:build integration

package jkl_test

// Note: Use go test -testwork to preserve the TestScript temporary directory.

import (
	"os"
	"testing"

	"github.com/ivanfetch/jkl"
	"github.com/rogpeppe/go-internal/testscript"
)

func init() {
	// Enable debugging for all tests, via the same environment variable the jkl
	// binary uses.
	if os.Getenv("JKL_DEBUG") != "" {
		jkl.EnableDebugOutput()
	}
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
	})
}
