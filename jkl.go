package jkl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	flag "github.com/spf13/pflag"
)

var debugLog *log.Logger = log.New(io.Discard, "", 0)

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

// New constructs a new JKL, accepting optional parameters via With*()
// functional options.
func New(options ...JKLOption) (*JKL, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot get executable to determine its parent directory: %v", err)
	}
	CWD := filepath.Dir(executable)
	absInstallsDir, err := filepath.Abs(CWD + "/../jkl-installs")
	if err != nil {
		return nil, fmt.Errorf("cannot get absolute path for installs location %q: %v", CWD+"/../jkl-installs", err)
	}
	j := &JKL{
		installsDir: absInstallsDir,
		// the default is to manage shims in the same directory jkl has been
		// installed.
		shimsDir:   CWD,
		executable: executable,
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

// RunCLI determines how this binary was run, and either calls RunShim() or
// processes JKL commands and arguments.
func (j JKL) RunCLI(args []string) error {
	calledProgName := filepath.Base(args[0])
	if calledProgName != callMeProgName { // Running as a shim
		return j.RunShim(args)
	}
	// Otherwise, running as jkl (not as a shim).
	errOutput := os.Stderr
	fs := flag.NewFlagSet(callMeProgName, flag.ExitOnError)
	fs.SetOutput(errOutput)
	fs.Usage = func() {
		fmt.Fprintf(errOutput, `%s manages command-line tools and their versions.

Usage: %s [flags]

Available command-line flags:
`,
			callMeProgName, callMeProgName)
		fs.PrintDefaults()
	}

	CLIVersion := fs.BoolP("version", "v", false, "Display the version and git commit.")
	CLIDebug := fs.BoolP("debug", "D", false, "Enable debug output")
	CLIInstall := fs.StringP("install", "i", "", "Download and install a tool. E.G. github.com/Owner/Repo or github.com/Owner/Repo:VersionTag")
	err := fs.Parse(args[1:])
	if err != nil {
		return err
	}

	if *CLIVersion {
		fmt.Fprintf(errOutput, "%s version %s, git commit %s\n", callMeProgName, Version, GitCommit)
		os.Exit(0)
	}
	if *CLIDebug || os.Getenv("JKL_DEBUG") != "" {
		EnableDebugOutput()
	}
	if *CLIInstall != "" {
		_, err := j.Install(*CLIInstall)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunShim executes the desired version of the tool which JKL was called as a
// shim, passing the remaining command-line arguments to the actual tool being
// executed.
func (j JKL) RunShim(args []string) error {
	if os.Getenv("JKL_DEBUG") != "" {
		EnableDebugOutput()
	}
	calledProgName := filepath.Base(args[0])
	desiredVersion, ok, err := j.getDesiredVersionForCommand(calledProgName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(`please specify which version of %s you would like to run, by setting the %s environment variable to a valid version, or to "latest" to use the latest version already installed.`, calledProgName, j.getDesiredCommandVersionEnvVarName(calledProgName))
	}
	installedCommandPath, err := j.getInstalledCommandPath(calledProgName, desiredVersion)
	if err != nil {
		return err
	}
	if installedCommandPath == "" {
		// Can we install the command version at this point? We don't know where the
		// command came from. LOL
		return fmt.Errorf("Version %s of %s is not installed", desiredVersion, calledProgName)
	}
	err = RunCommand(append([]string{installedCommandPath}, args[1:]...))
	if err != nil {
		return err
	}
	return nil
}

// Install installs the specified tool-specification and creates a shim,
// returning the version that was installed. The tool-specification represents where a tool can be
// downloaded, and an optional version.
func (j JKL) Install(toolSpec string) (installedVersion string, err error) {
	debugLog.Printf("Installing %s\n", toolSpec)
	var downloadPath, binaryPath, toolVersion string
	toolSpecFields := strings.Split(toolSpec, ":")
	g, err := NewGithubRepo(toolSpecFields[0])
	if err != nil {
		return "", err
	}
	if len(toolSpecFields) == 2 && strings.ToLower(toolSpecFields[1]) != "latest" {
		downloadPath, toolVersion, err = g.DownloadReleaseForVersion(toolSpecFields[1])
	} else {
		downloadPath, toolVersion, err = g.DownloadReleaseForLatest()
	}
	if err != nil {
		return "", err
	}
	err = ExtractFile(downloadPath)
	if err != nil {
		return "", err
	}
	toolName := filepath.Base(g.GetOwnerAndRepo())
	extractedToolBinary := fmt.Sprintf("%s/%s", filepath.Dir(downloadPath), toolName)
	installDest := fmt.Sprintf("%s/%s/%s", j.installsDir, toolName, toolVersion)
	err = CopyExecutableToCreatedDir(extractedToolBinary, installDest)
	if err != nil {
		return "", err
	}
	binaryPath = fmt.Sprintf("%s/%s", installDest, toolName)
	err = j.createShim(filepath.Base(binaryPath))
	if err != nil {
		return "", err
	}
	debugLog.Printf("Installed version %q", toolVersion)
	return toolVersion, nil
}

// CreateShim creates a symbolic link for the specified tool name, pointing to
// the JKL binary.
func (j JKL) createShim(binaryName string) error {
	debugLog.Printf("Creating shim %s -> %s\n", binaryName, j.executable)
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
	// ToDo: Check whether this exists and that it's correct, before attempting
	// to create it.
	err = os.Symlink(j.executable, fmt.Sprintf("%s/%s", j.shimsDir, binaryName))
	if err != nil {
		return err
	}
	return nil
}

// getInstalledCommandPath returns the full path to the desired version of the
// specified command. The version is obtained from configuration files or the
// command-specific environment variable.
func (j JKL) getInstalledCommandPath(commandName, commandVersion string) (installedCommandPath string, err error) {
	installedCommandPath = fmt.Sprintf("%s/%s/%s/%s", j.installsDir, commandName, commandVersion, commandName)
	_, err = os.Stat(installedCommandPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if errors.Is(err, fs.ErrNotExist) {
		debugLog.Printf("desired installed command %q not found", installedCommandPath)
		return "", nil
	}
	debugLog.Printf("installed command path for %s is %q\n", commandName, installedCommandPath)
	return installedCommandPath, nil
}

// getDesiredVersionForCommand returns the version of the specified command
// that is defined in configuration files or the command-specific environment
// variable. IF the specified version is `latest`, the latest installed
// version will be returned.
func (j JKL) getDesiredVersionForCommand(commandName string) (commandVersion string, found bool, err error) {
	envVarName := j.getDesiredCommandVersionEnvVarName(commandName)
	commandVersion = os.Getenv(envVarName)
	if commandVersion == "" {
		debugLog.Printf("environment variable %q is not set, looking in config files for the %s version", envVarName, commandName)
		var ok bool
		// ToDo: Our own config file is not yet implemented.
		commandVersion, ok, err = FindASDFToolVersion(commandName)
		if err != nil {
			return "", false, err
		}
		if !ok {
			debugLog.Printf("No version specified for command %q", commandName)
			return "", false, nil
		}
	}
	debugLog.Printf("version %s specified for %q\n", commandVersion, commandName)
	if strings.ToLower(commandVersion) == "latest" {
		return j.getLatestInstalledVersionForCommand(commandName)
	}
	return commandVersion, true, nil
}

// getDesiredCommandVersionEnvVarName returns the name of the environment
// variable that JKL will use to determine the desired version for a specified
// command.
func (j JKL) getDesiredCommandVersionEnvVarName(commandName string) string {
	// ToDo: Make this env var format configurable in the constructor?
	return fmt.Sprintf("JKL_%s", strings.ToUpper(strings.ReplaceAll(commandName, "-", "_")))
}

func (j JKL) getLatestInstalledVersionForCommand(commandName string) (commandVersion string, found bool, err error) {
	fileSystem := os.DirFS(filepath.Join(j.installsDir, commandName))
	versions := make([]string, 0)
	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != "." && d.IsDir() {
			versions = append(versions, path)
			return nil
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		debugLog.Printf("no versions found for command %q while looking for the latest installed version", commandName)
		return "", false, nil
	}
	commandVersion = versions[len(versions)-1]
	debugLog.Printf("the latest installed version of %s is %s", commandName, commandVersion)
	return commandVersion, true, nil
}
