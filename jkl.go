package jkl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	hashicorpversion "github.com/hashicorp/go-version"
	homedir "github.com/mitchellh/go-homedir"
)

var debugLog *log.Logger = log.New(io.Discard, "", 0)

var defaultHTTPClient http.Client = http.Client{Timeout: time.Second * 30}

const (
	callMeProgName = "jkl"
)

// JKL holds configuration.
type JKL struct {
	installsDir string // where downloaded tools are installed
	shimsDir    string // where shim symlinks are created
	executable  string // path to the jkl binary
}

func EnableDebugOutput() {
	debugLog.SetOutput(os.Stdout)
	debugLog.SetPrefix(callMeProgName + ": ")
}

// JKLOption uses a function  to set fields on a type JKL by operating on
// that type as an argument.
// This provides optional configuration and minimizes required parameters for
// the constructor.
type JKLOption func(*JKL) error

// WithInstallsDir sets the corresponding field in a JKL type.
func WithInstallsDir(d string) JKLOption {
	return func(j *JKL) error {
		if d == "" {
			return errors.New("the installs directory cannot be empty")
		}
		expandedD, err := homedir.Expand(d)
		if err != nil {
			return err
		}
		j.installsDir = expandedD
		return nil
	}
}

// WithShimsDir sets the corresponding field in a JKL type.
func WithShimsDir(d string) JKLOption {
	return func(j *JKL) error {
		if d == "" {
			return errors.New("the shims directory cannot be empty")
		}
		expandedD, err := homedir.Expand(d)
		if err != nil {
			return err
		}
		j.shimsDir = expandedD
		return nil
	}
}

// NewJKL constructs a new JKL instance, accepting optional parameters via With*()
// functional options.
func NewJKL(options ...JKLOption) (*JKL, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot get executable to determine its parent directory: %v", err)
	}
	j := &JKL{
		executable: executable,
	}
	// Use functional options to set default values.
	setDefaultInstallsDir := WithInstallsDir("~/.jkl/installs")
	err = setDefaultInstallsDir(j)
	if err != nil {
		return nil, err
	}
	setDefaultShimsDir := WithShimsDir("~/.jkl/bin")
	err = setDefaultShimsDir(j)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		err := option(j)
		if err != nil {
			return nil, err
		}
	}
	return j, nil
}

// GetExecutable returns the executable field from a type JKL.
func (j JKL) GetExecutable() string {
	return j.executable
}

func (j JKL) displayPreFlightCheck(output io.Writer) error {
	debugLog.Println("starting pre-flight check")
	shimsDirInPath, err := directoryInPath(j.shimsDir)
	if err != nil {
		fmt.Printf("Unable to verify whether the directory %q is in your PATH: %v\n", j.shimsDir, err)
	}
	if !shimsDirInPath {
		fmt.Fprintf(output, `WARNING: Please add the directory %[2]q to your PATH environment variable, so that %[1]s-managed tools can be run automatically.
Be sure the updated path takes effect by restarting your shell or sourcing the shell initialization file.
For example, you might add the following line to one of your shell initialization files:
PATH=%[2]q:$PATH
export PATH

`, callMeProgName, j.shimsDir)
		return err // potentially set by directoryInPath
	}
	debugLog.Println("pre-flight check done")
	return nil
}

func (j JKL) displayGettingStarted(output io.Writer) error {
	managedTools, err := j.listInstalledTools()
	if err != nil {
		return err
	}
	var numToolsPhrase string
	switch len(managedTools) {
	case 0:
		numToolsPhrase = "not yet managing any tools"
	case 1:
		numToolsPhrase = "already managing one tool"
	default:
		numToolsPhrase = fmt.Sprintf("already managing %d tools", len(managedTools))
	}
	fmt.Fprintf(output, `%[1]s is %[2]s.
To install more tools, run: %[1]s install github:<Github user>/<Github repository>
To list jkl-managed tools, run: %[1]s list
To list installed versions of a tool, run: %[1]s list <ToolName>
For additional help, run: %[1]s help
`, callMeProgName, numToolsPhrase)
	return nil
}

// Install installs the specified tool-specification and creates a shim,
// returning the version that was installed. The tool-specification represents
// the tool provider and an optional version.
func (j JKL) Install(toolSpec string) (installedVersion string, err error) {
	debugLog.Printf("Installing tool specification %q\n", toolSpec)
	var toolProvider, toolSource, toolVersion string
	toolSpecFields := strings.Split(toolSpec, ":")
	if len(toolSpecFields) > 3 {
		return "", fmt.Errorf("The tool specification %q has too many components - please supply a colon-separated provider, source, and optional version.", toolSpec)
	}
	if len(toolSpecFields) < 2 {
		return "", fmt.Errorf("the tool specification %q does not have enough components - please supply a colon-separated provider, source, and optional version", toolSpec)
	}
	if len(toolSpecFields) == 3 {
		toolVersion = toolSpecFields[2]
	}
	toolProvider = strings.ToLower(toolSpecFields[0])
	toolSource = toolSpecFields[1]
	var toolName, actualToolVersion, downloadPath string
	switch toolProvider {
	case "github", "gh":
		g, err := NewGithubRepo(toolSource)
		if err != nil {
			return "", err
		}
		downloadPath, actualToolVersion, toolName, err = g.DownloadReleaseForVersion(toolVersion)
		if err != nil {
			return "", err
		}
	case "hashicorp", "hashi":
		h, err := NewHashicorpProduct(toolSource)
		if err != nil {
			return "", err
		}
		toolName = toolSource
		downloadPath, actualToolVersion, err = h.DownloadReleaseForVersion(toolVersion)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown tool provider %q", toolProvider)
	}
	wasExtracted, err := ExtractFile(downloadPath)
	if err != nil {
		return "", err
	}
	var extractedToolBinary string
	extractedToolBinary = downloadPath // non-archived binary
	if wasExtracted {
		extractedToolBinary = fmt.Sprintf("%s/%s", filepath.Dir(downloadPath), toolName)
	}
	installDest := fmt.Sprintf("%s/%s/%s/%s", j.installsDir, toolName, actualToolVersion, toolName)
	err = CopyExecutableToCreatedDir(extractedToolBinary, installDest)
	if err != nil {
		return "", err
	}
	err = j.createShim(toolName)
	if err != nil {
		return "", err
	}
	debugLog.Printf("Installed version %q", actualToolVersion)
	return actualToolVersion, nil
}

// CreateShim creates a symbolic link for the specified tool name, pointing to
// the JKL binary.
func (j JKL) createShim(binaryName string) error {
	debugLog.Printf("Assessing shim %s\n", binaryName)
	_, err := os.Stat(j.shimsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if errors.Is(err, fs.ErrNotExist) {
		debugLog.Printf("creating directory %q", j.shimsDir)
		err := os.MkdirAll(j.shimsDir, 0700)
		if err != nil {
			return err
		}
	}
	shimPath := filepath.Join(j.shimsDir, binaryName)
	shimStat, err := os.Lstat(shimPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("while looking for existing shim %s: %v", shimPath, err)
	}
	if errors.Is(err, fs.ErrNotExist) {
		debugLog.Printf("Creating shim %s -> %s\n", binaryName, j.executable)
		err = os.Symlink(j.executable, shimPath)
		if err != nil {
			return err
		}
		return nil
	}
	if shimStat.Mode()&fs.ModeSymlink == 0 {
		return fmt.Errorf("not overwriting existing incorrect shim %s which should be a symlink (%v), but is instead mode %v", shimPath, fs.ModeSymlink, shimStat.Mode())
	}
	shimDest, err := filepath.EvalSymlinks(shimPath)
	if err != nil {
		return fmt.Errorf("while dereferencing shim symlink %s: %v", shimPath, err)
	}
	if shimDest == j.executable {
		debugLog.Printf("shim for %s already exists", shimPath)
		return nil
	}
	return fmt.Errorf("shim %s already exists but points to %q", shimPath, shimDest)
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
	if len(toolNames) == 0 {
		return nil, nil
	}
	sort.Strings(toolNames)
	return toolNames, nil
}

func (j JKL) displayInstalledTools(output io.Writer) error {
	toolNames, err := j.listInstalledTools()
	if err != nil {
		return err
	}
	for _, v := range toolNames {
		fmt.Fprintln(output, v)
	}
	return nil
}

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

func (j JKL) displayInstalledVersionsOfTool(output io.Writer, toolName string) error {
	toolVersions, ok, err := j.listInstalledVersionsOfTool(toolName)
	if err != nil {
		return fmt.Errorf("cannot list installed versions of %s: %v", toolName, err)
	}
	if !ok {
		fmt.Fprintf(output, "%s is not installed\n", toolName)
		return nil
	}
	sort.Strings(toolVersions)
	for _, v := range toolVersions {
		fmt.Fprintln(output, v)
	}
	return nil
}

// getDesiredVersionOfTool returns the version of the specified command
// that is defined in configuration files or the command-specific environment
// variable. IF the specified version is `latest`, the latest installed
// version will be returned.
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
// command.
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
