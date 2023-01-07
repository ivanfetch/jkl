//go:build integration

package jkl_test

// Note: The go test -testwork flag preserves the TestScript temporary directory.

import (
	"os"
	"testing"

	"github.com/ivanfetch/jkl"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	// Map binary names called by TestScript scripts, to run jkl.
	// This causes TestScript to symlink these binary names, affectively doing the
	// work of jkl.createShim()
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"jkl": jkl.Main,
		// List tools that jkl will install in testdata/script/* tests.
		"gh":        jkl.Main,
		"kind":      jkl.Main,
		"prme":      jkl.Main,
		"terraform": jkl.Main,
	}))
}
func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
	})
}
