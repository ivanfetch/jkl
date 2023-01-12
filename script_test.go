//go:build integration

package jkl_test

// Note: The go test -testwork flag preserves the TestScript temporary directory.

import (
	"fmt"
	"os"
	"testing"

	"github.com/ivanfetch/jkl"
	"github.com/rogpeppe/go-internal/testscript"
)

var testScriptSetup func(*testscript.Env) error = func(e *testscript.Env) error {
	e.Vars = append(e.Vars, fmt.Sprintf("GH_TOKEN=%s", os.Getenv("GH_TOKEN")))
	return nil
}

func TestMain(m *testing.M) {
	// Map binary names called by TestScript scripts, to run jkl.
	// This causes TestScript to symlink these binary names, affectively doing the
	// work of jkl.createShim()
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"jkl": jkl.Main,
		// List tools that jkl will install in testdata/script/* tests.
		// Using `exec toolname` in a .txtar file that is not listed here, will
		// return a TestScript error like:
		//                 testscript.go:224: no txtar nor txt scripts found in dir testdata/script
		"gh":        jkl.Main,
		"helm":      jkl.Main,
		"jq":        jkl.Main,
		"kind":      jkl.Main,
		"prme":      jkl.Main,
		"terraform": jkl.Main,
		"vault":     jkl.Main,
	}))
}
func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:   "testdata/script",
		Setup: testScriptSetup,
	})
}

// TestUpdateSelf tests updating the jkl binary to the latest available
// release from Github.
// This test canot run in parallel with other TestScript tests, because it
// messes with the jkl binary while other tests are using the
// TestScript-managed symlink.
func TestScriptUpdateSelf(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:   "testdata/update-self",
		Setup: testScriptSetup,
	})
}
