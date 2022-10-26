package jkl

import (
	"fmt"
	"strings"
)

// ToolSpec holds information about a tool for a provider to download.
type ToolSpec struct {
	name         string
	version      string
	provider     string // E.G. github, hashicorp
	source       string // E.G. Github owner/repo, Hashicorp product
	downloadPath string
}

// NewToolSpec accepts a tool specification of the form provider:source:[version]
// and returns a type ToolSpec.
func (j JKL) NewToolSpec(toolSpec string) (ToolSpec, error) {
	t := ToolSpec{}
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
