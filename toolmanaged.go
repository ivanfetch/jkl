package jkl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// managedTool represents a tool that JKL has already installed.
type managedTool struct {
	name string
	jkl  *JKL
}

// getManagedTool returns a type managedTool, with its name set to the
// specified tool name.
func (j JKL) getManagedTool(name string) *managedTool {
	tool := &managedTool{
		name: name,
		jkl:  &j,
	}
	return tool
}

// run executes the desired version of the specified tool. The desired version is
// determined via user configuration.
func (t managedTool) Run(args []string) error {
	desiredVersion, ok, err := t.desiredVersion()
	if err != nil {
		return err
	}
	if !ok {
		availableVersions, foundAnyVersions, err := t.listInstalledVersions()
		if err != nil {
			return err
		}
		if foundAnyVersions && len(availableVersions) > 1 {
			return fmt.Errorf(`please specify which version of %s you would like to run, by setting the %s environment variable to a valid version, or to "latest" to use the latest installed version.`, t.name, t.envVarName())
		}
		if foundAnyVersions {
			desiredVersion = availableVersions[0]
			debugLog.Printf("selecting the only available version %s for tool %s", desiredVersion, t.name)
		}
	}
	installedCommandPath, ok, err := t.path(desiredVersion)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("version %s of %s is not installed by %[3]s, please see the `%[3]s install` command to install it", desiredVersion, t.name, callMeProgName)
	}
	err = RunCommand(append([]string{installedCommandPath}, args...))
	if err != nil {
		return err
	}
	return nil
}

// path returns the full path to the specified version of the
// managedTool.
func (t managedTool) path(version string) (installedPath string, versionWasFound bool, err error) {
	for i, possibleVersion := range []string{version, toggleVPrefix(version)} {
		installedPath = fmt.Sprintf("%[1]s/%[2]s/%[3]s/%[2]s", t.jkl.installsDir, t.name, possibleVersion)
		_, err = os.Stat(installedPath)
		if err == nil {
			debugLog.Printf("found installed path for %s %s: %q\n", t.name, version, installedPath)
			return installedPath, true, nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", false, err
		}
		if i == 1 && errors.Is(err, fs.ErrNotExist) { // last possible version not found
			debugLog.Printf("version %q of tool %q is not installed, path %q not found with and without a leading v in the version number", version, t.name, installedPath)
			return "", false, nil
		}
	}
	return "", false, fmt.Errorf("unexpected loop fall-through finding the path for %q version %q", t.name, version)
}

// uninstallVersion removes the specified version of the managed tool,
// including it's containing directory which is named after the version.
// No error is returned if the specified version is not found.
func (t managedTool) uninstallVersion(version string) error {
	binaryPath, versionFound, err := t.path(version)
	if err != nil {
		return err
	}
	if !versionFound {
		debugLog.Printf("version %s of %s is not found and cannot be uninstalled", version, t.name)
		return nil
	}
	debugLog.Printf("removing tool binary %s", binaryPath)
	err = os.Remove(binaryPath)
	if err != nil {
		return err
	}
	parentPath := filepath.Dir(binaryPath)
	debugLog.Printf("removing the versioned directory %q", parentPath)
	err = os.Remove(parentPath)
	if err != nil {
		return err
	}
	return nil
}

// desiredVersion returns the version of the specified tool desired by
// configuration files or an environment variable. IF the version is `latest`, the latest installed version will be returned.
func (t managedTool) desiredVersion() (desiredVersion string, found bool, err error) {
	envVarName := t.envVarName()
	desiredVersion = os.Getenv(envVarName)
	if desiredVersion == "" {
		debugLog.Printf("environment variable %q is not set, looking in config files for the desired %s version", envVarName, t.name)
		var ok bool
		// ToDo: Our own config file is not yet implemented.
		desiredVersion, ok, err = FindASDFToolVersion(t.name)
		if err != nil {
			return "", false, err
		}
		if !ok {
			debugLog.Printf("No desired version specified for %q", t.name)
			return "", false, nil
		}
	}
	debugLog.Printf("desired version %q specified for %s\n", desiredVersion, t.name)
	if strings.ToLower(desiredVersion) == "latest" {
		return t.latestInstalledVersion()
	}
	return desiredVersion, true, nil
}

// envVarName returns the name of the environment
// variable that JKL will use to determine the desired version for the specified
// tool.
func (t managedTool) envVarName() string {
	// ToDo: Make this env var format configurable in the JKL constructor?
	return fmt.Sprintf("JKL_%s", strings.ToUpper(strings.ReplaceAll(t.name, "-", "_")))
}

// listInstalledVersions returns a sorted list of installed versions for
// the specified tool. The newest version will be last in the slice.
func (t managedTool) listInstalledVersions() (versions []string, found bool, err error) {
	fileSystem := os.DirFS(filepath.Join(t.jkl.installsDir, t.name))
	versions = make([]string, 0)
	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != "." && d.IsDir() {
			versions = append(versions, path)
			found = true
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	sortedVersions := sortVersions(versions)
	return sortedVersions, true, nil
}

func (t managedTool) uninstallAllVersions() error {
	uninstallErrs := new(multierror.Error)
	allVersions, foundAnyVersions, err := t.listInstalledVersions()
	if err != nil {
		return err
	}
	if !foundAnyVersions {
		debugLog.Printf("no versions of %s are installed, nothing to uninstall", t.name)
		return nil
	}
	for _, ver := range allVersions {
		err := t.uninstallVersion(ver)
		if err != nil {
			debugLog.Printf("error uninstalling %s version %s: %v\n", t.name, ver, err)
			uninstallErrs = multierror.Append(uninstallErrs, fmt.Errorf("uninstalling %s %s: %v", t.name, ver, err))
		}
	}
	if len(uninstallErrs.Errors) > 0 {
		return uninstallErrs
	}
	topLevelToolDir := filepath.Join(t.jkl.installsDir, t.name)
	debugLog.Printf("removing top-level directory %s\n", topLevelToolDir)
	err = os.Remove(topLevelToolDir)
	if err != nil {
		// Do not return an error if non-jkl-managed files are present, but make
		// the condition discoverable if debug logging is enabled.
		debugLog.Printf("cannot remove directory %q after having removed %s: %v\n", topLevelToolDir, t.name, err)
	}
	shim := filepath.Join(t.jkl.shimsDir, t.name)
	debugLog.Printf("removing shim %s\n", shim)
	err = os.Remove(shim)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unable to remove shim %q while uninstalling all versions of %s: %v", shim, t.name, err)
	}
	return nil
}

// latestInstalledVersion returns the latest version number that is installed
// of the specified tool.
func (t managedTool) latestInstalledVersion() (latestVersion string, found bool, err error) {
	versions, ok, err := t.listInstalledVersions()
	if err != nil {
		return "", false, err
	}
	if !ok {
		debugLog.Printf("no versions found for %q while looking for the latest installed version", t.name)
		return "", false, nil
	}
	latestVersion = versions[len(versions)-1]
	debugLog.Printf("the latest installed version of %s is %s", t.name, latestVersion)
	return latestVersion, true, nil
}

// listInstalledTools returns a list of tools with at least one version
// installed.
// A missing JKL.installsDir is not an error and will return 0 tools
// installed.
func (j JKL) listInstalledTools() (toolNames []string, err error) {
	fileSystem := os.DirFS(j.installsDir)
	toolNames = make([]string, 0)
	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if !errors.Is(err, fs.ErrNotExist) && err != nil {
			return err
		}
		if path != "." && d.IsDir() {
			tool := j.getManagedTool(path)
			_, hasVersions, err := tool.listInstalledVersions()
			if err != nil {
				return err
			}
			if hasVersions {
				toolNames = append(toolNames, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(toolNames)
	return toolNames, nil
}
