package jkl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	hashicorpversion "github.com/hashicorp/go-version"
)

// stringContainsOneOf reports the first substring contained in s, returning
// true if a match is found, and the original substring that matched.
// Substrings are matched using lower-case.
func stringContainsOneOf(s, firstSubstr string, additionalSubstrs ...string) (match string, found bool) {
	allSubStrings := append([]string{firstSubstr}, additionalSubstrs...)
	LCS := strings.ToLower(s)
	for i := len(allSubStrings) - 1; i >= 0; i-- {
		matchedIndex := strings.Index(LCS, strings.ToLower(allSubStrings[i]))
		if matchedIndex > -1 {
			matchedStr := s[matchedIndex : matchedIndex+len(allSubStrings[i])]
			debugLog.Printf("matched substring %s at index %d in %s", matchedStr, matchedIndex, s)
			return matchedStr, true
		}
	}
	return "", false
}

// stringEqualFoldOneOf returns true if the string is case-insensitively equal
// to one of the matches.
func stringEqualFoldOneOf(s, firstMatchstr string, additionalMatchstrs ...string) bool {
	for _, matchstr := range append([]string{firstMatchstr}, additionalMatchstrs...) {
		if strings.EqualFold(s, matchstr) {
			return true
		}
	}
	return false
}

// toggleVPrefix returns the specified string after adding an missing `v`
// prefix, or removing an existing `v` prefix.
func toggleVPrefix(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "v") {
		r := s[1:]
		debugLog.Printf("%q without v = %q", s, r)
		return r
	}
	r := "v" + s
	debugLog.Printf("%q with v = %q\n", s, r)
	return r
}

// CopyFile copies the specified file to destDir. The copy does not inharit
// the permissions of the source file. If destDir does not exist, an error is
// returned.
func CopyFile(filePath, destDir string) error {
	_, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	_, err = os.Stat(destDir)
	if err != nil {
		return err
	}
	fileName := filepath.Base(filePath)
	s, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(destDir + "/" + fileName)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("Cannot write to %s: %v", fileName, err)
	}
	return nil
}

// CopyExecutableToCreatedDir copies the specified file to destFilePath, and sets
// permissions to 0755 so the resulting file is executable. IF destFilePath
// minus the file name does
// not exist, the directory wil be created.
func CopyExecutableToCreatedDir(sourceFilePath, destFilePath string) error {
	_, err := os.Stat(sourceFilePath)
	if err != nil {
		return err
	}
	debugLog.Printf("copying file %q to %q", sourceFilePath, destFilePath)
	destDir := filepath.Dir(destFilePath)
	_, err = os.Stat(destDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if errors.Is(err, fs.ErrNotExist) {
		debugLog.Printf("creating directory %q", destDir)
		err := os.MkdirAll(destDir, 0700)
		if err != nil {
			return err
		}
	}
	s, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(destFilePath)
	if err != nil {
		return err
	}
	err = d.Chmod(0755)
	if err != nil {
		return fmt.Errorf("cannot set mode on %s: %v", destFilePath, err)
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("Cannot write to %s: %v", destFilePath, err)
	}
	return nil
}

// listPathsByParent returns directories where the specified file name is
// found, starting in the specified startDirName and traversing parent
// directories, stopping after processing rootDirName.
func listPathsByParent(fileName, startDirName, rootDirName string) (paths []string, err error) {
	if startDirName == "" {
		return nil, fmt.Errorf("startDirName cannot be empty")
	}
	if rootDirName == "" {
		return nil, fmt.Errorf("rootDirName cannot be empty")
	}
	debugLog.Printf("Starting to list paths where %q is found from %q to %q, by parent", fileName, startDirName, rootDirName)
	// Evaluating symlinks allows comparing to os.GetWD() which dereferences links
	// The startDirName or rootDirName parameters may have been supplied via GetWD().
	startDirName, _ = filepath.EvalSymlinks(startDirName)
	rootDirName, _ = filepath.EvalSymlinks(rootDirName)
	var checkDir string = startDirName
	for {
		_, err = os.Stat(filepath.Join(checkDir, fileName))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			paths = append(paths, checkDir)
		}
		if checkDir == rootDirName {
			debugLog.Printf("Done listing paths for %q by parent, reached specified root %q", fileName, rootDirName)
			break
		}
		checkDir = filepath.Clean(filepath.Join(checkDir, ".."))
	}
	debugLog.Printf("File %s was found in these paths: %v\n", fileName, paths)
	return paths, nil
}

// directoryInPath returns true if the specified directory is among those in
// the PATH environment variable.
func directoryInPath(dirName string) (bool, error) {
	if dirName == "" {
		return false, nil
	}
	absDirName, err := filepath.Abs(dirName)
	if err != nil {
		return false, fmt.Errorf("cannot make %q absolute: %v", dirName, err)
	}
	pathComponents := filepath.SplitList(os.Getenv("PATH"))
	for _, component := range pathComponents {
		absComponent, err := filepath.Abs(component)
		if err != nil {
			return false, fmt.Errorf("cannot make path component %q absolute: %v", component, err)
		}
		if absComponent == absDirName {
			return true, nil
		}
	}
	return false, nil
}

func getAliasesForArchitecture(arch string) []string {
	archAliases := map[string][]string{
		// The `universal` alias should always be last in the list, this supports macOS
		// binaries for amd64 and arm64.
		"amd64": {"x86_64", "64bit", "64-bit", "universal"},
	}
	return archAliases[strings.ToLower(arch)]
}

func getAliasesForOperatingSystem(OS string) []string {
	OSAliases := map[string][]string{
		"darwin": {"macos", "osx", "apple-darwin"},
	}
	return OSAliases[strings.ToLower(OS)]
}

// SortVersions returns the slice of strings sorted as semver version numbers.
// Any empty strings are replaced with version 0.0.0 before being sorted, to
// retain the size of the slice.
func sortVersions(versions []string) []string {
	debugLog.Printf("sorting %d versions: %v", len(versions), versions)
	sortedVersions := make([]*hashicorpversion.Version, len(versions))
	for i, v := range versions {
		if v == "" {
			debugLog.Printf("WARNING: the version at index %d is an empty string, using 0.0.0 instead", i)
			v = "0.0.0"
		}
		hv, err := hashicorpversion.NewVersion(v)
		if err != nil {
			debugLog.Printf("using string-sort while listing installed versions - the version %q can't be converted to a version, probably because it starts with extraneous text: %v", v, err)
			sort.Strings(versions)
			return versions
		}
		sortedVersions[i] = hv
	}
	sort.Sort(hashicorpversion.Collection(sortedVersions))
	for i, v := range sortedVersions { // reorder the original version strings by hashicorpversion.Version order
		versions[i] = v.Original()
	}
	debugLog.Printf("sorted versions are: %v", versions)
	return versions
}
