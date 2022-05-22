# JKL - A Tool Version Manager

JKL is a version manager for other command-line tools. It installs tools quickly with minimal input, and helps you switch versions of tools while you work.

**JKL is a public work in progress - not all functionality is complete and there are plenty of rough edges.**

* Install a new command-line tool from its Github release or direct download URL.
	* Target a specific version (`v1.2.3`), `latest`, or the latest partial version (`v1.2` or `v1`).
	* Versions can match Github release tags with or without a leading `v`.
	* A Github asset is matched to your operating system and architecture.
	* It's ok if the tool is contained in a tar or zip archive.
* JKL creates a "shim" to intercept the execution of the just-installed tool, so that whenyou attempt to run the tool JKL can determine which version to run.
	* Specify which version of a given tool to run via an an environment variable, configuration file, or your shell current directory.
	* Specifying `latest` runs the latest installed version.
	* Defaults can be set by configuration files in higher-level parent directories. Child configuration files can specify only a tool's version, with parent configuration files specifying where that tool can be downloaded.
* Install multiple tools in parallel - useful when bootstrapping a new workstation or standard versions of tooling used by a project.

## JKL Installation

This process is mostly incomplete as I experiment for the best user experience. The intent is:

* Download a Github release or build JKL on your own if desired.
* Put the `jkl` binary in your `$PATH`, ideally the same location where you would like JKL to create shims for JKL-managed tools.
* Optionally override the directory where JKL manages tools that it installs. This defaults to `~/.jkl/installs`
* Use JKL to install your first tool by running `jkl -i github:User/Repo` (replacing `User` and `Repo` with a Github user and repository).

##Features Under Consideration

These are features or user experience that need more consideration.

* JKL configuration files will specify the "provider" and desired version of a tool. The provider represents where / how to download the tool (`github`, `URLTemplate`, `CurlBash`).
	* A provider may not need to be specified in all config files. Config files can be read from parent directories to find a tool's provider. This could allow a project/environment to specify desired tool versions without needing to care about the provider.
* A JKL setup / init command that uses JKL to manage itself.
* A central "for all users" operating mode to support shared environments like jump-boxes:
	* Avoid each user needing to install their own copies of common tools.
	* Allow users to install new tools or versions not already present in a shared location.
	* Try hard to not become a full-fledged package manager. :)
* Support additional features via "plugins" - such as:
	* Some tools will require post-install action, like managing a shell initialization file.
	* Some tools will have multiple binaries, like Go, Python or other runtimes.
	* Some logic may be required depending on architecture or to generate default configuration for a tool.
* Use user-installed tools, instead of JKL-managed ones.
	* The user-installed tools would follow a configurable naming convention such as `tool.x.y.z` or `tool-x.y.z`.
	* The first binary found in the PATH matching the naming convention would be used.
* A `cleanup` option that uninstalls versions of tools that aren't referenced in config files within a directory tree.
* A `nuke` option that uninstalls everything JKL manages.
* A bulk purge option to remove all tools from a particular provider, or Github user.
