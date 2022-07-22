package jkl_test

import (
	"testing"

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
			ReleaseName: "1.0.3-rc1",
			TagName:     "1.0.3-rc1",
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
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			gotTag, gotMatch := fakeGithubReleases.MatchTagFromPartialVersion(tc.version)
			if tc.expectMatch && !gotMatch {
				t.Fatal("expected version to match a tag")
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
