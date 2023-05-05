package jkl

import (
	"fmt"
	"strings"
)

// ToolSpec holds information about a tool for a provider to process.
type ToolSpec struct {
	name             string
	version          string
	provider         string // E.G. github, hashicorp
	source           string // E.G. Github owner/repo, Hashicorp product
	downloadPath     string // downloaded tool, populated by the provider
	verifyDownload   bool   // VErify the checksum of downloadpath, if supported by the provider.
	checksumFilePath string // A checksums file populated by the provider, used to verify downloadPath
}

// NewToolSpec accepts a tool specification and whether to verify the download
// checksum, returning a type ToolSpec.
// THe tool specification is of the form provider:source:[version]
func (j JKL) NewToolSpec(toolSpec string, verifyDL bool) (ToolSpec, error) {
	t := ToolSpec{
		verifyDownload: verifyDL,
	}
	toolSpecFields := strings.Split(toolSpec, ":")
	if len(toolSpecFields) > 3 {
		return t, fmt.Errorf("The tool specification %q has too many components - please supply a colon-separated provider, source, and optional version.", toolSpec)
	}
	if len(toolSpecFields) < 2 {
		return t, fmt.Errorf("the tool specification %q does not have enough components - please supply a colon-separated provider, source, and optional version", toolSpec)
	}
	if len(toolSpecFields) == 3 {
		t.version = toolSpecFields[2]
	}
	t.provider = strings.ToLower(toolSpecFields[0])
	t.source = toolSpecFields[1]
	return t, nil
}
