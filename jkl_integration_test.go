//go:build integration

package jkl_test

import (
	"fmt"
	"github.com/ivanfetch/jkl"
	"io/fs"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInstall(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description        string
		toolSpec           string
		wantInstalledFiles []string
		wantShims          []string
		wantVersion        string
		expectError        bool
	}{
		{
			description:        "latest version of ivanfetch/prme",
			toolSpec:           "ivanfetch/prme",
			wantVersion:        "v0.0.6",
			wantInstalledFiles: []string{"prme/v0.0.6/prme"},
			wantShims:          []string{"prme"},
		},
		{
			description:        "version v0.0.4 of ivanfetch/prme",
			toolSpec:           "ivanfetch/prme:0.0.4",
			wantVersion:        "v0.0.4",
			wantInstalledFiles: []string{"prme/v0.0.4/prme"},
			wantShims:          []string{"prme"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tempDir := t.TempDir()
			j, err := jkl.New(jkl.WithInstallsDir(tempDir+"/installs"), jkl.WithShimsDir(tempDir+"/shims"))
			if err != nil {
				t.Fatal(err)
			}
			gotVersion, err := j.Install(tc.toolSpec)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantVersion != gotVersion {
				t.Fatalf("want version %q, got %q", tc.wantVersion, gotVersion)
			}
			gotInstalledFiles, err := filesInDir(tempDir + "/installs")
			if err != nil {
				t.Fatalf("listing installed files: %v", err)
			}
			sort.Strings(gotInstalledFiles)
			if !cmp.Equal(tc.wantInstalledFiles, gotInstalledFiles) {
				t.Fatalf("want vs. got installed files: %s", cmp.Diff(tc.wantInstalledFiles, gotInstalledFiles))
			}
			gotShims, err := filesInDir(tempDir + "/shims")
			if err != nil {
				t.Fatalf("listing shims: %v", err)
			}
			sort.Strings(gotShims)
			if !cmp.Equal(tc.wantShims, gotShims) {
				t.Fatalf("want vs. got shims %s", cmp.Diff(tc.wantShims, gotShims))
			}
			for _, shim := range gotShims {
				shimStat, err := os.Lstat(fmt.Sprintf("%s/shims/%s", tempDir, shim))
				if err != nil {
					t.Fatalf("getting file info for shim %s in %s: %v", shim, tempDir, err)
				}
				if shimStat.Mode()&fs.ModeSymlink == 0 {
					t.Fatalf("want shim %s to be a symlink (%v), but got mode %v", shim, fs.ModeSymlink, shimStat.Mode())
				}
			}
		})
	}
}
