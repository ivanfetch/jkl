package jkl

import (
	"fmt"
)

/*
Helm lists releases on Github, but hosts binariesvia the get.helm.sh CDN.
The binaries are linked from the github releases page, but are not listed
as github assets. :(
The current approach to install Helm uses some Github code to determine
available versions and match tags, then construct a get.helm.sh download
URL of the form: https://get.helm.sh/helm-v{version}-${GOOS}-{GOARCH}.tar.gz
*/

// HelmDownload accepts a type toolSpec and populates it with the path of the
// downloaded file.
// The toolSpec may also be updated with the
// version of Helm that was downloaded, in cases where a partial or
// "latest" version is specified.
func HelmDownload(TS *ToolSpec) error {
	g, err := NewGithubRepo("helm/helm")
	if err != nil {
		return err
	}
	tag, ok, err := g.findTagForVersion(TS.version)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no github tag found matching helm version %q", TS.version)
	}
	binaryPath, err := g.DownloadHelmBinaryForTag(tag)
	if err != nil {
		return err
	}
	TS.name = "helm"
	TS.version = tag
	TS.downloadPath = binaryPath
	return nil
}
