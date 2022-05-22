package jkl_test

import (
	"errors"
	"fmt"
	"io/fs"
	"jkl"
	"os"
	"path/filepath"
	"testing"
)

func TestFindASDFToolVersion(t *testing.T) {
	// These are non-parallel tests because they change the current working
	// directory.
	testCases := []struct {
		description             string
		testCWD                 string // within test tempDir
		toolName                string
		toolVersionFilesContent map[string]string // paths to .tool-versions files and content
		wantVersion             string
		expectFound             bool
		expectError             bool
	}{
		{
			description:             "app 1.2.3 in the current directory",
			testCWD:                 ".",
			toolName:                "app",
			toolVersionFilesContent: map[string]string{".": "app 1.2.3"},
			wantVersion:             "1.2.3",
			expectFound:             true,
		},
		{
			description: "app 1.2.3 2 sub-dirs deep",
			testCWD:     "./dir2/dir3",
			toolName:    "app",
			toolVersionFilesContent: map[string]string{
				".":           "app 1.1.1",
				"./dir2":      "app 1.2.2",
				"./dir2/dir3": "app 1.2.3"},
			wantVersion: "1.2.3",
			expectFound: true,
		},
		{
			description: "app 1.2.3 in sub-dir with child different app",
			testCWD:     "./dir2/dir3",
			toolName:    "app",
			toolVersionFilesContent: map[string]string{
				".":           "app 1.1.1",
				"./dir2":      "app 1.2.3",
				"./dir2/dir3": "differentapp 1.2.3"},
			wantVersion: "1.2.3",
			expectFound: true,
		},
		{
			description: "app not listed in any tools-versions files",
			testCWD:     "./dir2/dir3/dir4",
			toolName:    "app",
			toolVersionFilesContent: map[string]string{
				".":                "dummy 1.1.1",
				"./dir2":           "anotherDummy 1.5.0",
				"./dir2/dir3":      "yetAnotherDummy 1.0.0",
				"./dir2/dir3/dir4": "aFinalDummy 0.4.0",
			},
			expectFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tempDir := t.TempDir()
			testDir := tempDir + "/" + tc.testCWD
			oldCWD, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				dErr := os.Chdir(oldCWD)
				if dErr != nil {
					err = dErr
				}
			}()
			for subDir, fileContent := range tc.toolVersionFilesContent {
				fileName := fmt.Sprintf("%s/%s/.tool-versions", tempDir, subDir)
				err := writeDirsAndFile(fileName, fileContent)
				if err != nil {
					t.Fatalf("writing test tool-versions file %s: %v", fileName, err)
				}
			}
			err = os.Chdir(testDir)
			if err != nil {
				t.Fatalf("unable to change to test-case directory %q: %v", testDir, err)
			}
			gotToolVersion, gotOK, err := jkl.FindASDFToolVersion(tc.toolName, jkl.WithAlternateRootDir(tempDir))
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatal("an error is expected")
			}
			if tc.expectFound != gotOK {
				t.Fatalf("expected tool version to be found, but it was not found")
			}
			if tc.wantVersion != gotToolVersion {
				t.Fatalf("want tool version %q, got %q", tc.wantVersion, gotToolVersion)
			}
		})
	}
}

func writeDirsAndFile(filePath, fileContent string) error {
	dir := filepath.Dir(filePath)
	_, err := os.Stat(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("varifying directory %s exists: %v", dir, err)
	}
	if errors.Is(err, fs.ErrNotExist) {
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			return fmt.Errorf("while creating directory %s: %v", dir, err)
		}
	}
	err = os.WriteFile(filePath, []byte(fileContent), 0600)
	if err != nil {
		return err
	}
	return nil
}
