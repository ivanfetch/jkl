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
	"strings"
	"time"

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
func (j JKL) Install(specStr string) (installedVersion string, err error) {
	debugLog.Printf("Installing tool specification %q\n", specStr)
	toolSpec, err := j.NewToolSpec(specStr)
	if err != nil {
		return "", err
	}
	switch toolSpec.provider {
	case "github", "gh":
		var err error
		switch strings.ToLower(toolSpec.source) {
		case "helm/helm":
			err = HelmDownload(&toolSpec)
		default:
			err = GithubDownload(&toolSpec)
		}
		if err != nil {
			return "", err
		}
	case "hashicorp", "hashi":
		err := HashicorpDownload(&toolSpec)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown tool provider %q", toolSpec.provider)
	}
	wasExtracted, err := ExtractFile(toolSpec.downloadPath)
	if err != nil {
		return "", err
	}
	var finalBinary string
	if wasExtracted {
		finalBinary = fmt.Sprintf("%s/%s", filepath.Dir(toolSpec.downloadPath), toolSpec.name)
		debugLog.Printf("using extracted binary %q for tool %s\n", finalBinary, toolSpec.name)
	} else {
		finalBinary = toolSpec.downloadPath
		debugLog.Printf("using non-extracted binary %q for tool %s\n", finalBinary, toolSpec.name)
	}
	installDest := fmt.Sprintf("%s/%s/%s/%s", j.installsDir, toolSpec.name, toolSpec.version, toolSpec.name)
	err = CopyExecutableToCreatedDir(finalBinary, installDest)
	if err != nil {
		return "", err
	}
	err = j.createShim(toolSpec.name)
	if err != nil {
		return "", err
	}
	debugLog.Printf("Installed %s version %q", toolSpec.name, toolSpec.version)
	return toolSpec.version, nil
}

// Uninstall uninsalls the specified managedTool. All versions will be
// uninstalled unless a version is specified.
func (j JKL) Uninstall(toolNameAndVersion string) error {
	toolFields := strings.Split(toolNameAndVersion, ":")
	toolName := toolFields[0]
	var toolVersion string
	if len(toolFields) == 2 {
		toolVersion = toolFields[1]
	}
	tool := j.getManagedTool(toolName)
	if toolVersion == "" {
		debugLog.Printf("uninstalling all versions of %s", toolName)
		return tool.uninstallAllVersions()
	}
	err := tool.uninstallVersion(toolVersion)
	if err != nil {
		return err
	}
	return nil
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
		debugLog.Printf("creating shims directory %q", j.shimsDir)
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
	// The shim did not need to be created, verify it is correct.
	if shimStat.Mode()&fs.ModeSymlink == 0 {
		return fmt.Errorf("not overwriting existing incorrect shim %s which should be a symlink (%v), but is instead mode %v", shimPath, fs.ModeSymlink, shimStat.Mode())
	}
	shimDest, err := filepath.EvalSymlinks(shimPath)
	if err != nil {
		return fmt.Errorf("while dereferencing shim symlink %s: %v", shimPath, err)
	}
	shimDestStat, err := os.Stat(shimDest)
	if err != nil {
		return err
	}
	executableStat, err := os.Stat(j.executable)
	if err != nil {
		return err
	}
	if os.SameFile(shimDestStat, executableStat) {
		debugLog.Printf("shim for %s already exists", shimPath)
		return nil
	}
	return fmt.Errorf("shim %s already exists but points to %q instead of %q", shimPath, shimDest, j.executable)
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

func (j JKL) displayInstalledVersionsOfTool(output io.Writer, toolName string) error {
	tool := j.getManagedTool(toolName)
	toolVersions, ok, err := tool.listInstalledVersions()
	if err != nil {
		return fmt.Errorf("cannot list installed versions of %s: %v", toolName, err)
	}
	if !ok {
		fmt.Fprintf(output, "%s is not installed\n", toolName)
		return nil
	}
	for _, v := range toolVersions {
		fmt.Fprintln(output, v)
	}
	return nil
}
