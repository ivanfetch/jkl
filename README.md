# JKL - A Tool Version Manager

Jkl is a version manager for other command-line tools. It installs tools quickly with minimal input, and helps you switch versions of tools while you work.

**Jkl is a public work in progress - not all of the below functionality is complete and there are plenty of rough edges.**

## Getting Started

* Install jkl using one of the following:
	* Use Homebrew: `brew tap ivanfetch/jkl && brew install jkl`
	* Manually download [a jkl release](https://github.com/ivanfetch/jkl/releases)
	* Use the Go install command: `go install github.com/ivanfetch/jkl/cmd/jkl@latest`
	* Build jkl from a clone of this repository by running `git clone https://github.com/ivanfetch/jkl && cd jkl && go build cmd/jkl`
* Run `jkl` to performa pre-fight check. This will instruct you to add the `~/.jkl/bin` directory to your path, and install your first jkl-managed tool using a command like: `jkl install github:<github user>/<github repository>` or `jkl install hashicorp:<product name>`

## Features and How It Works

* Install a new command-line tool from its Github release, a [Hashicorp product](https://www.hashicorp.com/), or other sources (see [features under consideration](#features-under-consideration) below
	* Specify an optional version to be installed like `v1.2.3`), `latest`, or the latest partial version (`v1.2` or `v1`).
	* Versions can match with or without a leading `v`.
	* A download is matched to your operating system and architecture.
	* The download can be a single binary, or be contained in a tar or zip archive.
* Jkl creates a "shim" to intercept the execution of managed tools, so jkl can determine which version you want to run.
	* Specify which version of an installed tool to run via an an environment variable, configuration file, or your shell current directory. Only use of an environment variable is currently implemented.
	* Specifying a version of `latest` runs the latest installed version of a tool.
	* Defaults can be set by a configuration file in the current or in parent directories. Child configuration files can specify only a tool's version, with parent configuration files specifying where that tool can be downloaded.
* Install multiple tools in parallel - useful when bootstrapping a new workstation or standard versions of tooling used by a project.

## Features Under Consideration

These features  need more consideration, but are documented here as they evolve.

* Additional providers, such as
	* `go install` (using Golang to build/install a tool)
	* A download URL template, including a separate URL to obtain the latest available version of that tool. This provider likely can't support installing the latest major or minor version.
* Jkl configuration files will specify the "provider" and desired version of a tool. The provider represents where / how to download the tool (`github`, `URLTemplate`, `CurlBash`).
	* A provider may not need to be specified in all config files. Config files can be read from parent directories to find a tool's provider. This could allow a project/environment to specify desired tool versions without needing to care about the provider.
* A central; shared operating mode to support environments like jump-boxes:
	* Avoid each user needing to install their own copies of common tools.
	* Allow users to install new tools or versions not already present in a shared location.
	* Try hard to not become a full-fledged package manager. :)
* Upgrade all managed tools to the latest patch release, or `x.y` or `x` semver version.
	* How should jkl remember the provider that was used when a tool was installed?
* Jkl self-upgrade via command like `jkl update`.
	* Should JKL manage (multiple versions of) itself?
* Support additional features via "plugins" - such as:
	* Some tools will require post-install action, like managing a shell initialization file.
	* Some tools will have multiple binaries, like Go, Python, or other runtimes.
	* Some install-time logic may be required depending on architecture or to generate default configuration for a tool.
* Use user-installed tools, instead of jkl-managed ones.
	* The user-installed tools would follow a configurable naming convention such as `tool.x.y.z` or `tool-x.y.z`.
	* The first binary found in the PATH matching the naming convention would be used.
* A `cleanup` option that uninstalls versions of tools that aren't referenced in config files within a directory tree.
* A `nuke` option that uninstalls everything jkl manages.
* A bulk purge option to remove all tools from a particular provider, or Github user.
