package jkl_test

import (
	"jkl"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func init() {
	// Enable debugging for all tests, via the same environment variable the jkl
	// binary uses.
	if os.Getenv("JKL_DEBUG") != "" {
		jkl.EnableDebugOutput()
	}
}

func TestMatchGithubAsset(t *testing.T) {
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

	got, ok := jkl.MatchAssetByOsAndArch(testAssets, "darwin", "amd64")
	want := jkl.GithubAsset{
		Name: "prme_0.0.6_Darwin_x86_64.tar.gz",
		URL:  "https://api.github.com/repos/ivanfetch/PRMe/releases/assets/47905345",
	}
	if !ok {
		t.Fatal("no asset matched")
	}
	if !cmp.Equal(want, got) {
		t.Fatalf("want vs. got: %s", cmp.Diff(want, got))
	}
}
