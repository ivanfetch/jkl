package jkl_test

import (
	"fmt"
	"jkl"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// wrapRunCommand is a helper function that calls jkl.RunCommand() without
// allowing its syscall.Exec() to replace the go test binary.
// It accomplishes this by forking a new process running the same go test
// binary, targeting an explicit test that will call jkl.RunCommand().
func wrapRunCommand(command []string) error {
	cmd := exec.Command(os.Args[0], "-test.run=TestExecHelper")
	cmd.Env = append(os.Environ(), "test_exec_helper_command="+strings.Join(command, " "), "test_exec_helper_explicit=true")
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, o)
	}
	return nil
}

// TestExecHelper is run by wrapRunCommand() to facilitate calling
// jkl.RunCommand() in a sub-process.
func TestExecHelper(t *testing.T) {
	calledExplicitly := os.Getenv("test_exec_helper_explicit")
	if calledExplicitly != "true" { // Avoid this test running on its own
		return
	}
	commandString := os.Getenv("test_exec_helper_command")
	err := jkl.RunCommand(strings.Split(commandString, " "))
	if err != nil {
		t.Fatal(err)
	}
	t.Errorf("RunCommand() did not return an error, but also did not syscall.Exec() for command %q", commandString)
}

func TestRunCommandToTouchAFile(t *testing.T) {
	// t.Parallel()
	path := t.TempDir() + "/" + t.Name()
	command := []string{"/usr/bin/touch", path}
	err := wrapRunCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindCommandVersion(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PATH", tempDir)
	want := tempDir + "/testcommand.1.0"
	f, err := os.Create(want)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = f.Chmod(0755)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jkl.FindCommandVersion("testcommand", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}
