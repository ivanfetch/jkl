package jkl_test

import (
	"io/fs"
	"jkl"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExtractFile(t *testing.T) {
	// These are non-parallel tests because they change the current working
	// directory.
	// Eventually code will use all absolute filesystem locations and these
	// tests can be parallelized.
	testCases := []struct {
		description     string
		archiveFilePath string
		extractedFiles  []string
		expectError     bool
	}{
		{
			description:     "Single file gzip compressed",
			archiveFilePath: "file.gz",
			extractedFiles:  []string{"file"},
		},
		{
			description:     "Single file bzip2 compressed",
			archiveFilePath: "file.bz2",
			extractedFiles:  []string{"file"},
		},
		{
			description:     "tar gzip compressed",
			archiveFilePath: "file.tar.gz",
			extractedFiles:  []string{"file", "subdir/file"},
		},
		{
			description:     "tar bzip2 compressed",
			archiveFilePath: "file.tar.bz2",
			extractedFiles:  []string{"file", "subdir/file"},
		},
		{
			description:     "uncompressed tar",
			archiveFilePath: "file.tar",
			extractedFiles:  []string{"file", "subdir/file"},
		},
		{
			description:     "zip",
			archiveFilePath: "file.zip",
			extractedFiles:  []string{"file", "subdir/file"},
		},
		{
			description:     "Truncated gzip which will return an error",
			archiveFilePath: "truncated.gz",
			extractedFiles:  []string{},
			expectError:     true,
		},
		{
			description:     "Truncated bzip2 which will return an error",
			archiveFilePath: "truncated.bz2",
			extractedFiles:  []string{},
			expectError:     true,
		},
		{
			description:     "Truncated tar which will return an error",
			archiveFilePath: "truncated.tar",
			extractedFiles:  []string{"file"}, // This will partially extract.
			expectError:     true,
		},
		{
			description:     "Truncated zip which will return an error",
			archiveFilePath: "truncated.zip",
			extractedFiles:  []string{},
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tempDir := t.TempDir()
			err := jkl.CopyFile("testdata/archives/"+tc.archiveFilePath, tempDir)
			if err != nil {
				t.Fatal(err)
			}
			tempArchiveFilePath := tempDir + "/" + filepath.Base(tc.archiveFilePath)
			err = jkl.ExtractFile(tempArchiveFilePath)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			// Include the archive file in the list of expected files, which was
			// also 			copied into tempDir.
			wantExtractedFiles := make([]string, len(tc.extractedFiles)+1)
			copy(wantExtractedFiles, tc.extractedFiles)
			wantExtractedFiles[len(wantExtractedFiles)-1] = tc.archiveFilePath
			sort.Strings(wantExtractedFiles)
			gotExtractedFiles, err := filesInDir(tempDir)
			if err != nil {
				t.Fatalf("listing files that were extracted: %v", err)
			}
			if !cmp.Equal(wantExtractedFiles, gotExtractedFiles) {
				t.Fatalf("want vs. got files extracted: %s", cmp.Diff(wantExtractedFiles, gotExtractedFiles))
			}
		})
	}
}

// filesInDir returns the sorted list of recursive files contained in the
// specified directory.
func filesInDir(dir string) ([]string, error) {
	fileSystem := os.DirFS(dir)
	files := make([]string, 0)
	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." || d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
