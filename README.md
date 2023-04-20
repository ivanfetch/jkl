# JKL - A Tool Version Manager

Jkl is a version manager for other command-line tools. It installs tools quickly with minimal input, and helps you switch versions of tools while you work.

**Jkl is a public work in progress - not all of the below functionality is complete and there are still rough edges to be found.**

## Getting Started

### Installation

Install jkl using one of the following:

* Use Homebrew: `brew tap ivanfetch/jkl && brew install jkl`
* Manually download [a jkl release](https://github.com/ivanfetch/jkl/releases), and copy it to a directory in your `$PATH`
* Use the Go install command: `go install github.com/ivanfetch/jkl/cmd/jkl@latest`
* Build jkl from a clone of this repository by running `git clone https://github.com/ivanfetch/jkl && cd jkl && go build cmd/jkl`

### Usage

Run `jkl` to performa pre-fight check. This will instruct you to add the `~/.jkl/bin` directory to your path, and install your first jkl-managed tool using the `jkl install` command - for example:

* `jkl install github:<github user>/<github repository>`
* `jkl install hashicorp:<product name>`

The `jkl version` command will alert when a new version is available, and the `jkl update` command can be used to update the jkl binary. It is not yet possible to upgrade tools that are managed by jkl.

See the output of `jkl --help` for more detail about how to use it.

## Features and How It Works

Some of the below is noted as "not yet implemented," but is included to paint a complete picture of where jkl is going.

* Install a new command-line tool from its Github release, a [Hashicorp product](https://www.hashicorp.com/), or other sources (see [features under consideration](#features-under-consideration) below
	* Specify an optional version of the tool to be installed, for example `latest`, `v1.2.3`, or the latest major or minor version like `v1.2` or `v1`. If no version is specified, the latest version is installed.
	* Versions will be matched with or without a leading `v` character.
	* A download is matched to your operating system and architecture.
	* The download can be a single binary, or be contained in a tar or zip archive.
* Jkl creates a "shim" to intercept the execution of the tools it manages, so jkl can determine which version of a tool you want to run.
	* Specify which version of an installed tool to run via an an environment variable, configuration file, or your shell current directory. Only use of an environment variable is currently implemented.
	* Specifying a version of `latest` runs the latest installed version of a tool.
	* Defaults can be set by a configuration file in the current or in parent directories. Child configuration files can specify only a tool's version, with parent configuration files specifying where that tool can be downloaded. The configuration file is not yet implemented.
* Install multiple tools in parallel - useful when bootstrapping a new workstation or standard versions of tooling used by a project. Installation of multiple tools at a time is not yet implemented.

## Features Under Consideration

These features  need more consideration, but are documented here as they evolve.

* Additional providers, such as
	* `go install` (using Golang to build/install a tool)
	* A download URL template, including a separate URL to obtain the latest available version of that tool. E.G. installing `kubectl`. This provider likely can't support installing the latest major or minor version.
* Jkl configuration files will specify the "provider" and desired version of a tool. The provider represents where / how to download the tool (`github`, `hashicorp`, `URLTemplate`, `CurlBash`).
	* A provider may not need to be specified in all config files. Config files can be read from parent directories to find a tool's provider. This could allow a project/environment to specify desired tool versions without needing to care about the provider.
* A central; shared operating mode to support environments like jump-boxes:
	* Avoid each user needing to install their own copies of common tools.
	* Allow users to install new tools or versions not already present in a shared location.
	* Try hard to not become a full-fledged package manager. :)
* Upgrade all managed tools to the latest patch release, or `x.y` or `x` semver version.
	* How should jkl remember the provider that was used when a tool was installed?
* Support additional features via "plugins" - such as:
	* Some tools will require post-install action, like managing a shell initialization file.
	* Some tools will have multiple binaries, like Go, Python, or other runtimes.
	* Some install-time logic may be required depending on architecture or to generate default configuration for a tool.
* Use already-installed binaries, instead of jkl-managed ones.
	* The user-installed tools would follow a configurable naming convention such as `tool.x.y.z` or `tool-x.y.z`.
	* The first binary found in the PATH matching the naming convention would be used.
* A `cleanup` option that uninstalls versions of tools that aren't referenced in config files within a directory tree.
* A `nuke` option that uninstalls everything jkl manages.
* A bulk purge option to remove all tools from a particular provider, or Github user.
