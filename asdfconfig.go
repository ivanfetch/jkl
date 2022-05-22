package jkl

import (
	"bufio"
	"os"
	"strings"
)

const (
	ASDFConfigFileName = ".tool-versions"
)

// findASDFToolVersion traverses parent directories to find the desired
// version for the specified tool, in the ASDF configuration file.
func FindASDFToolVersion(toolName string, locationOptions ...pathOption) (toolVersion string, foundTool bool, err error) {
	locations, err := listPathsByParent(ASDFConfigFileName, locationOptions...)
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
