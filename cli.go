package jkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Main is an exported function that calls the same entrypoint as the jkl
// binary. This is used by tests, to run jkl. See script_test.go.
func Main() (exitCode int) {
	err := RunCLI(os.Args, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

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

	var versionOnly, commitOnly bool
	var versionCmd = &cobra.Command{
		Use:     "version",
		Short:   "Display the jkl version and git commit",
		Long:    "Display the jkl version and git commit",
		Aliases: []string{"ver", "v"},
		Run: func(cmd *cobra.Command, args []string) {
			if versionOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", Version)
				return
			}
			if commitOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", GitCommit)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s version %s, git commit %s\n", callMeProgName, Version, GitCommit)
		},
	}
	versionCmd.Flags().BoolVarP(&versionOnly, "version-only", "v", false, "Only output the jkl version.")
	versionCmd.Flags().BoolVarP(&commitOnly, "commit-only", "c", false, "Only output the jkl git commit.")
	versionCmd.MarkFlagsMutuallyExclusive("version-only", "commit-only")
	rootCmd.AddCommand(versionCmd)

	var installCmd = &cobra.Command{
		Use:   "install <provider>:<source>[:version]",
		Short: "Install a command-line tool",
		Long: `Install a command-line tool.

	If no version is specified, the latest version will be installed (not including pre-release versions). A partial major version will match the latest minor one.

Available providers are:
	github|gh - install a Github release. The source is specified as <Github user>/<Github repository>.
	hashicorp|hashi - install a Hashicorp product. The source is the name of the Hashicorp product.`,
		Example: `	jkl install github:fairwindsops/rbac-lookup
	jkl install github:fairwindsops/rbac-lookup:0.9.0
	jkl install github:fairwindsops/rbac-lookup:0.8
	jkl install hashicorp:terraform:1.2`,
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

	var uninstallCmd = &cobra.Command{
		Use:   "uninstall <tool name>[:version]",
		Short: "Uninstall a command-line tool",
		Long: `Uninstall a command-line tool managed by JKL.

	A tool version must be exact, as shown by: jkl list <tool name>
	If no version is specified, all versions of the tool will be uninstalled.`,
		Example: `	jkl uninstall rbac-lookup
	jkl uninstall rbac-lookup:0.9.0`,
		Aliases: []string{"remove", "uninst", "u", "rm"},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Please specify which tool JKL should uninstall, using the form: ToolName[:version]\nThe `jkl list` command will show JKL-managed tools. Run %s uninstall -h for more information about uninstallation.", callMeProgName)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := j.Uninstall(args[0])
			if err != nil {
				return err
			}
			return nil
		},
	}
	rootCmd.AddCommand(uninstallCmd)

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

// RunShim executes the desired version of the tool which the JKL shim was called.
// The remaining command-line arguments are passed to the actual tool being
// executed.
func (j JKL) RunShim(args []string) error {
	if os.Getenv("JKL_DEBUG") != "" {
		EnableDebugOutput()
	}
	calledProgName := filepath.Base(args[0])
	tool := j.getManagedTool(calledProgName)
	err := tool.Run(args[1:])
	if err != nil {
		return err
	}
	return nil
}
