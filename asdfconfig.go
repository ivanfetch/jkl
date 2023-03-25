package jkl

import (
	"bufio"
	"os"
	"strings"
)

const (
	ASDFConfigFileName = ".tool-versions"
)

// asdfConfigSearchParameters holds the start and end directory
// boundaries to use when searching for an ASDF configuration file.
type asdfConfigSearchParameters struct {
	startDir string // Name of the directory to begin searching for an ASDF config file
	rootDir  string // Where to stop traversing parent directories while searching for an ASDF config file
}

// asdfConfigSearchOption is the functional options pattern for
// asdfConfigSearchParameters
type asdfConfigSearchOption func(*asdfConfigSearchParameters)

// WithASDFConfigSearchStartDir sets a root directory where
// FindASDFToolVersion() should start searching for an ASDF configuration file.
func WithASDFConfigSearchStartDir(s string) asdfConfigSearchOption {
	return func(p *asdfConfigSearchParameters) {
		p.startDir = s
	}
}

// WithASDFConfigSearchRootDir sets a root directory where
// FindASDFToolVersion() should stop searching for an ASDF configuration file.
func WithASDFConfigSearchRootDir(r string) asdfConfigSearchOption {
	return func(p *asdfConfigSearchParameters) {
		p.rootDir = r
	}
}

// findASDFToolVersion traverses parent directories to find the desired
// version for the specified tool, in the ASDF configuration file.
// The WithASDFConfigSearch* functions can be used to specify th start and
// stop (root) directory where config files should be consulted.
func FindASDFToolVersion(toolName string, asdfConfigSearchOptions ...asdfConfigSearchOption) (toolVersion string, foundTool bool, err error) {
	searchParams := &asdfConfigSearchParameters{}
	for _, option := range asdfConfigSearchOptions {
		option(searchParams)
	}
	if searchParams.startDir == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return "", false, err
		}
		searchParams.startDir = currentDir
	}
	if searchParams.rootDir == "" {
		searchParams.rootDir = "/"
	}
	locations, err := listPathsByParent(ASDFConfigFileName, searchParams.startDir, searchParams.rootDir)
	if err != nil {
		return "", false, err
	}
	for _, location := range locations {
		v, ok, err := getToolVersionFromASDFConfigFile(location+"/"+ASDFConfigFileName, toolName)
		if err != nil {
			return "", false, err
		}
		if ok {
			return v, true, nil
		}
	}
	return "", false, nil
}

// getToolVersionFromASDFConfigFile parses an ASDF tool-versions configuration
// file, returning the version for the specified tool, if found.
func getToolVersionFromASDFConfigFile(filePath, toolName string) (toolVersion string, foundTool bool, err error) {
	debugLog.Printf("Reading ASDF config file %s for %s version", filePath, toolName)
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Split(bufio.ScanLines)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "" {
			continue
		}
		if string(fields[0][0]) == "#" {
			continue
		}
		if len(fields) < 2 {
			debugLog.Printf("No version found in line: %q\n", s.Text())
			continue
		}
		if len(fields) > 2 {
			debugLog.Printf("Too many tokens found in line: %q\n", s.Text())
			continue
		}
		if fields[0] == toolName {
			toolVersion = fields[1]
			debugLog.Printf("Found version %s for %s in ASDF config file %s", toolVersion, toolName, filePath)
			return toolVersion, true, nil
		}
	}
	return "", false, nil
}
