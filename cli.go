package jkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// RunCLI determines how this binary was run, and either calls RunShim() or
// processes JKL commands and arguments.
func RunCLI(args []string, output, errOutput io.Writer) error {
	j, err := NewJKL()
	if err != nil {
		return err
	}
	calledProgName := filepath.Base(args[0])
	if calledProgName != callMeProgName { // Running as a shim
		return j.RunShim(args)
	}

	// Cobra commands are defined here to inharit the JKL instance.
	var debugFlagEnabled bool
	var rootCmd = &cobra.Command{
		Use:           "jkl",
		Short:         "A command-line tool version manager",
		Long:          `JKL is a version manager for other command-line tools. It installs tools quickly with minimal input, and helps you switch versions of tools while you work.`,
		SilenceErrors: true, // will be bubbled up and output elsewhere
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getenv("JKL_DEBUG") != "" || debugFlagEnabled {
				EnableDebugOutput()
			}
			err := j.displayPreFlightCheck(cmd.OutOrStdout())
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := j.displayGettingStarted(cmd.OutOrStdout())
			return err
		},
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true // Until completion behavior is tested
	rootCmd.PersistentFlags().BoolVarP(&debugFlagEnabled, "debug", "D", false, "Enable debug output (also enabled by setting the JKL_DEBUG environment variable to any value).")

	var versionCmd = &cobra.Command{
		Use:     "version",
		Short:   "Display the jkl version and git commit",
		Long:    "Display the jkl version and git commit",
		Aliases: []string{"ver", "v"},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s version %s, git commit %s\n", callMeProgName, Version, GitCommit)
		},
	}
	rootCmd.AddCommand(versionCmd)
	var installCmd = &cobra.Command{
		Use:   "install <provider>:<source>[:version]",
		Short: "Install a command-line tool",
		Long: `Install a command-line tool.

	If no version is specified, the latest version will be installed (not including pre-release versions). A partial major version will match the latest minor one.

Available providers are:
	github|gh - install a Github release. The source is specified as <Github user>/<Github repository>.`,
		Example: `	jkl install github:fairwindsops/rbac-lookup
jkl install github:fairwindsops/rbac-lookup:0.9.0
	jkl install github:fairwindsops/rbac-lookup:0.8`,
		Aliases: []string{"add", "inst", "i"},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Please specify what you would like to install, using a colon-separated provider, source, and optional version. Run %s install -h for more information about installation providers, and matching tool versions.", callMeProgName)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := j.Install(args[0])
			if err != nil {
				return err
			}
			return nil
		},
	}
	rootCmd.AddCommand(installCmd)

	var listCmd = &cobra.Command{
		Use:   "list [<tool name>]",
		Short: "List installed command-line tools or installed versions for a specific tool",
		Long: `List command-line tools that jkl has installed.

With no arguments, all tools that jkl has installed are shown. With a tool name, jkl lists installed versions of that tool.`,
		Example: `	jkl list
jkl list rbac-lookup`,
		Aliases: []string{"ls", "lis", "l"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				err := j.displayInstalledVersionsOfTool(cmd.OutOrStdout(), args[0])
				return err
			}
			err := j.displayInstalledTools(cmd.OutOrStdout())
			return err
		},
	}
	rootCmd.AddCommand(listCmd)

	cobra.CheckErr(rootCmd.Execute())
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
	desiredVersion, ok, err := j.getDesiredVersionOfTool(calledProgName)
	if err != nil {
		return err
	}
	if !ok {
		availableVersions, foundVersions, err := j.listInstalledVersionsOfTool(calledProgName)
		if err != nil {
			return err
		}
		if foundVersions && len(availableVersions) > 1 {
			return fmt.Errorf(`please specify which version of %s you would like to run, by setting the %s environment variable to a valid version, or to "latest" to use the latest version already installed.`, calledProgName, j.getEnvVarNameForToolDesiredVersion(calledProgName))
		}
		if foundVersions {
			desiredVersion = availableVersions[0]
			debugLog.Printf("selecting only available version %s for tool %s", desiredVersion, calledProgName)
		}
	}
	installedCommandPath, err := j.getPathForToolDesiredVersion(calledProgName, desiredVersion)
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
