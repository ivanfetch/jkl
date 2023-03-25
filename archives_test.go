package jkl_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ivanfetch/jkl"

	"github.com/google/go-cmp/cmp"
)

func TestExtractFile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description     string
		archiveFilePath string
		extractedFiles  []string
		wasExtracted    bool
		expectError     bool
	}{
		{
			description:     "Single file gzip compressed",
			archiveFilePath: "file.gz",
			extractedFiles:  []string{"file"},
			wasExtracted:    true,
		},
		{
			description:     "Single file bzip2 compressed",
			archiveFilePath: "file.bz2",
			extractedFiles:  []string{"file"},
			wasExtracted:    true,
		},
		{
			description:     "tar gzip compressed",
			archiveFilePath: "file.tar.gz",
			extractedFiles:  []string{"file", "file2"},
			wasExtracted:    true,
		},
		{
			description:     "tar bzip2 compressed",
			archiveFilePath: "file.tar.bz2",
			extractedFiles:  []string{"file", "file2"},
			wasExtracted:    true,
		},
		{
			description:     "uncompressed tar",
			archiveFilePath: "file.tar",
			extractedFiles:  []string{"file", "file2"},
			wasExtracted:    true,
		},
		{
			description:     "zip",
			archiveFilePath: "file.zip",
			extractedFiles:  []string{"file", "file2"},
			wasExtracted:    true,
		},
		{
			description:     "A plain file not in an archive",
			archiveFilePath: "plain-file",
			extractedFiles:  []string{"plain-file"},
			wasExtracted:    false,
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
		tc := tc // Capture range variable
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			err := jkl.CopyFile("testdata/archives/"+tc.archiveFilePath, tempDir)
			if err != nil {
				t.Fatal(err)
			}
			tempArchiveFilePath := tempDir + "/" + filepath.Base(tc.archiveFilePath)
			wasExtracted, err := jkl.ExtractFile(tempArchiveFilePath)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if tc.wasExtracted != wasExtracted {
				t.Errorf("want wasExtracted to be %v, got %v", tc.wasExtracted, wasExtracted)
			}
			// IF files are expected to be extracted, include the archive file in the
			// list of expected files, which was
			// also 			copied into tempDir.
			wantExtractedFiles := make([]string, len(tc.extractedFiles))
			copy(wantExtractedFiles, tc.extractedFiles)
			if tc.wasExtracted || tc.expectError {
				wantExtractedFiles = append(wantExtractedFiles, tc.archiveFilePath)
			}
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
