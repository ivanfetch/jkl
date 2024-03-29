package jkl_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ivanfetch/jkl"
)

func TestGithubMatchTagFromPartialVersion(t *testing.T) {
	t.Parallel()
	fakeGithubReleases := jkl.GithubReleases{
		{
			ReleaseName: "0.8",
			TagName:     "0.8",
		},
		{
			ReleaseName: "0.9",
			TagName:     "0.9",
		},
		{
			ReleaseName: "1.0.0",
			TagName:     "1.0.0",
		},
		{
			ReleaseName: "1.0.2",
			TagName:     "1.0.2",
		},
		{
			ReleaseName: "release with no version",
			TagName:     "",
		},
		{
			ReleaseName: "1.0.3-rc1",
			TagName:     "1.0.3-rc1",
			PreRelease:  true,
		},
		{
			ReleaseName: "2.0.1", // skipped 2.0.0
			TagName:     "2.0.1",
		},
		{
			ReleaseName: "3.0.0",
			TagName:     "3.0.0",
		},
		{
			ReleaseName: "3.0.1",
			TagName:     "3.0.1",
		},
		{
			ReleaseName: "3.0.2",
			TagName:     "3.0.2",
		},
		{
			ReleaseName: "3.0.3",
			TagName:     "3.0.3",
		},
		{
			ReleaseName: "jq 1.6",
			TagName:     "jq-1.6",
		},
	}

	testCases := []struct {
		description string
		version     string
		wantTag     string
		expectMatch bool
	}{
		{
			description: "match tag 3.0.3 from partial version 3.0",
			version:     "3.0",
			wantTag:     "3.0.3",
			expectMatch: true,
		},
		{
			description: "match tag 1.0.2 from partial version 1",
			version:     "1",
			wantTag:     "1.0.2",
			expectMatch: true,
		},
		{
			description: "match tag 2.0.1 from partial version 2.0",
			version:     "2.0",
			wantTag:     "2.0.1",
			expectMatch: true,
		},
		{
			description: "match tag (with extraneous text) jq-1.6 from partial version 1.6",
			version:     "1.6",
			wantTag:     "jq-1.6",
			expectMatch: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			gotTag, gotMatch := fakeGithubReleases.MatchTagFromPartialVersion(tc.version)
			if tc.expectMatch && !gotMatch {
				t.Fatal("expected version to match a tag, try running tests with the JKL_DEBUG environment variable set for more information")
			}
			if !tc.expectMatch && gotMatch {
				t.Fatalf("unexpectedly matched tag %q to version %q\n", gotTag, tc.version)
			}
			if tc.wantTag != gotTag {
				t.Fatalf("Want tag %q, got %q\n", tc.wantTag, gotTag)
			}
		})
	}
}

func TestGithubAssetNameWithoutVersionAndComponents(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description      string
		asset            jkl.GithubAsset
		removeComponents []string
		want             string
	}{
		{
			description:      "archived binary with version, OS, and architecture",
			asset:            jkl.GithubAsset{Name: "app_v1.2.3_darwin_x64.tar.gz"},
			removeComponents: []string{"darwin", "x64"},
			want:             "app",
		},
		{
			description:      "not-archived binary with no version",
			asset:            jkl.GithubAsset{Name: "app-darwin-amd64"},
			removeComponents: []string{"darwin", "amd64"},
			want:             "app",
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			got := tc.asset.NameWithoutVersionAndComponents(tc.removeComponents...)
			if tc.want != got {
				t.Errorf("want base name %q, got %q for asset %q", tc.want, got, tc.asset.Name)
			}
		})
	}
}

func TestMatchAssetByOsAndArch(t *testing.T) {
	t.Parallel()

	testAssets := []jkl.GithubAsset{
		{
			Name: "checksums.txt",
			URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905347",
		},
		{
			Name: "prme_0.0.6_Darwin_x86_64.tar.gz",
			URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905345",
		},
		{
			Name: "prme_0.0.6_Linux_arm64.tar.gz",
			URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905348",
		},
		{
			Name: "prme_0.0.6_Linux_x86_64.tar.gz",
			URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905353",
		},
		{
			Name: "prme_0.0.6_Windows_x86_64.tar.gz",
			URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905349",
		},
	}

	gotAsset, gotOS, gotArch, ok := jkl.MatchAssetByOsAndArch(testAssets, "darwin", "amd64")
	wantAsset := jkl.GithubAsset{
		Name: "prme_0.0.6_Darwin_x86_64.tar.gz",
		URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905345",
	}
	if !ok {
		t.Fatal("no asset matched")
	}
	if !cmp.Equal(wantAsset, gotAsset) {
		t.Fatalf("want vs. got: %s", cmp.Diff(wantAsset, gotAsset))
	}
	wantOS := "Darwin"
	if wantOS != gotOS {
		t.Fatalf("want OS %s, got %s", wantOS, gotOS)
	}
	wantArch := "x86_64"
	if wantArch != gotArch {
		t.Fatalf("want architecture %s, got %s", wantArch, gotArch)
	}
}
