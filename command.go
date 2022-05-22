package jkl

import (
	"os"
	"os/exec"
	"syscall"
)

// RunCommand execs the specified command, the command will replace the
// current (Go program) process. If commandAndArgs[0] is not an absolute path,
// the PATH environment variable will be searched for the executable.
func RunCommand(commandAndArgs []string) error {
	cmd, err := exec.LookPath(commandAndArgs[0])
	if err != nil {
		return err
	}
	debugLog.Printf("Going to exec %s which was in path for command %v\n", cmd, commandAndArgs[0])
	err = syscall.Exec(cmd, commandAndArgs, os.Environ())
	if err != nil {
		return err
	}
	return nil
}

// FindCommandVersion accepts a command name and desired x.y.z version,
// returning the path to the command, otherwise
// returning an empty string if the command is not found in the PATH
// environment variable.
// This is used to find user-installed binaries. JKL-installed binaries are
// instead located within the JKL `installs` directory, using the function
// getInstalledCommandPath().
func FindCommandVersion(command, version string) (string, error) {
	commandWithVersion := command + "." + version
	found, err := exec.LookPath(commandWithVersion)
	execErr, ok := err.(*exec.Error)
	if ok && execErr.Err == exec.ErrNotFound {
		debugLog.Printf("did not find command %s version %s\n", command, version)
		return "", nil
	}
	if err != nil {
		return "", err
	}
	debugLog.Printf("found %q for command %s version %s\n", found, command, version)
	return found, nil
}
