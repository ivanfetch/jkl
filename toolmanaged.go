package jkl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	hashicorpversion "github.com/hashicorp/go-version"
)

// getPathForToolDesiredVersion returns the full path to the desired version of the
// specified command. The version is obtained from configuration files or the
// command-specific environment variable.
func (j JKL) getPathForToolDesiredVersion(toolName, toolVersion string) (installedPath string, err error) {
	installedPath = fmt.Sprintf("%[1]s/%[2]s/%[3]s/%[2]s", j.installsDir, toolName, toolVersion)
	_, err = os.Stat(installedPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if errors.Is(err, fs.ErrNotExist) {
		debugLog.Printf("desired installed tool %q not found", installedPath)
		return "", nil
	}
	debugLog.Printf("installed path for %s is %q\n", toolName, installedPath)
	return installedPath, nil
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
			_, hasVersions, err := j.listInstalledVersionsOfTool(path)
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

// listInstalledVersionsOfTool returns a list of installed versions for
// the specified tool.
func (j JKL) listInstalledVersionsOfTool(toolName string) (versions []string, found bool, err error) {
	fileSystem := os.DirFS(filepath.Join(j.installsDir, toolName))
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
	return versions, true, nil
}

// getDesiredVersionOfTool returns the version of the specified tool according
// to configuration files or an environment variable. IF the specified version is `latest`, the latest installed version will be returned.
func (j JKL) getDesiredVersionOfTool(toolName string) (desiredVersion string, found bool, err error) {
	envVarName := j.getEnvVarNameForToolDesiredVersion(toolName)
	desiredVersion = os.Getenv(envVarName)
	if desiredVersion == "" {
		debugLog.Printf("environment variable %q is not set, looking in config files for the %s version", envVarName, toolName)
		var ok bool
		// ToDo: Our own config file is not yet implemented.
		desiredVersion, ok, err = FindASDFToolVersion(toolName)
		if err != nil {
			return "", false, err
		}
		if !ok {
			debugLog.Printf("No desired version specified for %q", toolName)
			return "", false, nil
		}
	}
	debugLog.Printf("version %s specified for %q\n", desiredVersion, toolName)
	if strings.ToLower(desiredVersion) == "latest" {
		return j.getLatestInstalledVersionOfTool(toolName)
	}
	return desiredVersion, true, nil
}

// getEnvVarNameForToolDesiredVersion returns the name of the environment
// variable that JKL will use to determine the desired version for a specified
// tool.
func (j JKL) getEnvVarNameForToolDesiredVersion(toolName string) string {
	// ToDo: Make this env var format configurable in the constructor?
	return fmt.Sprintf("JKL_%s", strings.ToUpper(strings.ReplaceAll(toolName, "-", "_")))
}

func (j JKL) getLatestInstalledVersionOfTool(toolName string) (latestVersion string, found bool, err error) {
	versions, ok, err := j.listInstalledVersionsOfTool(toolName)
	if err != nil {
		return "", false, err
	}
	if !ok {
		debugLog.Printf("no versions found for %q while looking for the latest installed version", toolName)
		return "", false, nil
	}
	sortedVersions := make([]*hashicorpversion.Version, len(versions))
	for i, v := range versions {
		hv, _ := hashicorpversion.NewVersion(v)
		sortedVersions[i] = hv
	}
	sort.Sort(hashicorpversion.Collection(sortedVersions))
	latestVersion = sortedVersions[len(sortedVersions)-1].Original()
	debugLog.Printf("the latest installed version of %s is %s", toolName, latestVersion)
	return latestVersion, true, nil
}
