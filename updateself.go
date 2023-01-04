package jkl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// UpdateSelf downloads the latest jkl binary and overwrites the currently
// executing one. The new binary is run, to verify it reports the expected
// newer version.
func (j JKL) UpdateSelf() (newVersion string, isNewerVersion bool, err error) {
	debugLog.Printf("updating %s from %s to the latest version", j.executable, Version)
	downloadedJKLPath, newVersion, isNewerVersion, err := j.DownloadAndExtractlaterJKLVersion()
	if err != nil {
		return
	}
	if !isNewerVersion {
		return
	}
	debugLog.Printf("downloaded jkl %s to %q\n", newVersion, downloadedJKLPath)
	versionReportedByNewBinary, err := getVersionOfJKLBinary(downloadedJKLPath)
	if err != nil {
		return newVersion, isNewerVersion, fmt.Errorf("while executing a newly downloaded JKL binary (%s version -v) to verify its version is %q: %v: %s", downloadedJKLPath, newVersion, err, versionReportedByNewBinary)
	}
	debugLog.Printf("the downloaded JKL binary reports version %q", versionReportedByNewBinary)
	if "v"+versionReportedByNewBinary != newVersion {
		return newVersion, isNewerVersion, fmt.Errorf("the newly downloaded JKL binary reports version %q instead of the expected %q", versionReportedByNewBinary, newVersion)
	}
	destDir := filepath.Dir(j.executable)
	debugLog.Printf("copying new JKL binary to %q\n", destDir)
	err = CopyFile(downloadedJKLPath, destDir)
	if err != nil {
		return newVersion, isNewerVersion, fmt.Errorf("while copying new JKL binary to %s: %v", destDir, err)
	}
	return
}

// DownloadAndExtractlaterJKLVersion downloads and extracts the latest version
// of jkl, if that version is newer than the currently executing one.
// The JKL binary will be set to the file-mode of the current binary.
// It returns the path to the downloaded binary, the latestversion number, and whether a
// newer version exists.
func (j JKL) DownloadAndExtractlaterJKLVersion() (binaryPath, matchedVersion string, newerVerAvailable bool, err error) {
	g, err := NewGithubRepo("ivanfetch/jkl")
	if err != nil {
		return
	}
	latestTag, err := g.GetTagForLatestRelease()
	if err != nil {
		return
	}
	if latestTag == "v"+Version {
		debugLog.Printf("while downloading an updated version of JKL, version %q is already the latest release", Version)
		return
	}
	newerVerAvailable = true
	downloadPath, _, err := g.DownloadReleaseForTag(latestTag)
	if err != nil {
		return
	}
	_, err = ExtractFile(downloadPath)
	if err != nil {
		return
	}
	existingJKLStat, err := os.Stat(j.executable)
	if err != nil {
		return
	}
	existingJKLFileMode := existingJKLStat.Mode()
	newJKLBinaryPath := filepath.Join(filepath.Dir(downloadPath), "jkl")
	err = os.Chmod(newJKLBinaryPath, existingJKLFileMode)
	if err != nil {
		return
	}
	return newJKLBinaryPath, latestTag, newerVerAvailable, nil
}

// getVersionOfJKLBinary runs the specified jkl binary to determine its
// version.
func getVersionOfJKLBinary(binaryPath string) (version string, err error) {
	cmd := exec.Command(binaryPath, "version", "-v") // returns only the version
	cmd.Env = append(os.Environ(), `JKL_DEBUG=`)     // debug output can obscure the version output
	outputBytes, err := cmd.CombinedOutput()
	returnedVersion := strings.TrimSuffix(string(outputBytes), "\n")
	if err != nil {
		return returnedVersion, err
	}
	return returnedVersion, nil
}
