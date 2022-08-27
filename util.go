package jkl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// StringContainsOneOf returns true if one of the sub-strings is contained
// within s.
func stringContainsOneOf(s, firstSubstr string, additionalSubstrs ...string) bool {
	for _, substr := range append([]string{firstSubstr}, additionalSubstrs...) {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
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

// pathOption is the functional options pattern for listPathsByParent.
type pathOption func(*string)

// WithAlternateRootDir sets a root directory other than "/", to instruct
// listPathsByParent() when to stop searching parent directories.
// listPathsByParent.
func WithAlternateRootDir(r string) pathOption {
	return func(d *string) {
		*d = r
	}
}

// listPathsByParent returns directories where the specified file name is
// found, starting in the current working directory and traversing parent
// directories until the root (/ or a specified alternative) is reached.
func listPathsByParent(fileName string, options ...pathOption) (paths []string, err error) {
	debugLog.Printf("Starting to list paths for %q by parent", fileName)
	var rootPath *string
	defaultRootPath := "/"
	rootPath = &defaultRootPath
	for _, option := range options {
		option(rootPath)
	}
	// Evaluating symlinks allows comparing to os.GetCWD() which dereferences links
	*rootPath, _ = filepath.EvalSymlinks(*rootPath)
	oldCWD, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer func() {
		dErr := os.Chdir(oldCWD)
		if dErr != nil { // avoid setting upstream err to nil
			err = dErr
		}
	}()
	for {
		CWD, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		_, err = os.Stat(fileName)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			paths = append(paths, CWD)
		}
		if CWD == *rootPath {
			debugLog.Printf("Done listing paths for %q by parent, reached specified root %q", fileName, *rootPath)
			break
		}
		err = os.Chdir("..")
		if err != nil {
			return nil, err
		}
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
