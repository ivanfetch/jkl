package jkl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type GithubAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GithubReleases []struct {
	ReleaseName string `json:"name"`
	TagName     string `json:"tag_name"`
}

func (g GithubReleases) tagExists(wantTag string) (tag string, found bool) {
	debugLog.Printf("Looking for tag %q in %d releases\n", wantTag, len(g))
	for _, r := range g {
		if strings.EqualFold(r.TagName, wantTag) {
			debugLog.Printf("found tag %q for release %s\n", r.TagName, r.ReleaseName)
			return r.TagName, true
		}
	}
	debugLog.Printf("tag %q not found\n", wantTag)
	return "", false
}

func (g GithubReleases) tagForReleaseName(wantName string) (tag string, found bool) {
	debugLog.Printf("Looking for name %q in %d releases\n", wantName, len(g))
	for _, r := range g {
		if strings.EqualFold(r.ReleaseName, wantName) {
			debugLog.Printf("found release name %s which has tag %q\n", r.ReleaseName, r.TagName)
			return r.TagName, true
		}
	}
	debugLog.Printf("name %q not found\n", wantName)
	return "", false
}

func (gr GithubReleases) MatchTagFromPartialVersion(pv string) (tag string, found bool) {
	debugLog.Printf("matching tag from partial version %q\n", pv)
	tags := make([]string, len(gr))
	for i, j := range gr {
		tags[i] = j.TagName
	}
	sort.Strings(tags)
	// Iterate the Github release tags backwards.
	for i := len(tags) - 1; i >= 0; i-- {
		LCPV := strings.ToLower(pv)
		LCThisTag := strings.ToLower(tags[i])
		if stringContainsOneOf(LCThisTag, "-rc", "-alpha", "-beta") {
			debugLog.Printf("skipping pre-release tag %q\n", tags[i])
			continue
		}
		if strings.HasPrefix(LCThisTag, LCPV) || strings.HasPrefix(LCThisTag, "v"+LCPV) {
			debugLog.Printf("matched tag %q for partial version %s\n", tags[i], pv)
			return tags[i], true
		}
	}
	debugLog.Printf("no partial match for %s\n", pv)
	return "", false
}

type Downloader struct {
	githubToken, githubAPIHost string
	httpClient                 *http.Client
}

func NewDownloader() *Downloader {
	return &Downloader{
		githubAPIHost: "https://api.github.com",
		githubToken:   os.Getenv("GH_TOKEN"),
		httpClient:    &http.Client{Timeout: time.Second * 30},
	}
}

func (d *Downloader) githubAPIRequest(method, URI string) (*http.Response, error) {
	if !strings.HasPrefix(URI, "/") {
		URI = "/" + URI
	}
	URL := d.githubAPIHost + URI
	req, err := http.NewRequest(method, URL, nil)
	if err != nil {
		return nil, err
	}
	if d.githubToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", d.githubToken))
	}
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d Downloader) ListGithubAssetsForTag(ownerAndRepo, tag string) ([]GithubAsset, error) {
	ok, err := d.GithubRepoExists(ownerAndRepo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("no such repository")
	}
	URI := "/repos/" + ownerAndRepo + "/releases/tags/" + tag
	resp, err := d.githubAPIRequest(http.MethodGet, URI)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
	}
	var APIResp struct {
		Assets []GithubAsset `json:"assets"`
	}
	err = json.NewDecoder(resp.Body).Decode(&APIResp)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if len(APIResp.Assets) == 0 {
		return nil, errors.New("the Github API did not return the expected fields")
	}
	return APIResp.Assets, nil
}

func (d Downloader) GetGithubLatestReleaseTag(ownerAndRepo string) (tagName string, err error) {
	ok, err := d.GithubRepoExists(ownerAndRepo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("no such repository")
	}
	URI := "/repos/" + ownerAndRepo + "/releases/latest"
	resp, err := d.githubAPIRequest(http.MethodGet, URI)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
	}
	var APIResp struct {
		TagName *string `json:"tag_name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&APIResp)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if APIResp.TagName == nil {
		return "", errors.New("the Github API did not return tag_name")
	}
	return *APIResp.TagName, nil
}

func (d Downloader) GithubRepoExists(ownerAndRepo string) (bool, error) {
	URI := "/repos/" + ownerAndRepo
	resp, err := d.githubAPIRequest(http.MethodGet, URI)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
}

func (d Downloader) Download(asset GithubAsset) (filePath string, err error) {
	req, err := http.NewRequest(http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	if d.githubToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", d.githubToken))
	}
	req.Header.Add("Accept", "application/octet-stream")
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, asset.URL)
	}
	tempDir, err := os.MkdirTemp(os.TempDir(), callMeProgName+"-")
	if err != nil {
		return "", err
	}
	filePath = fmt.Sprintf("%s/%s", tempDir, asset.Name)
	f, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return filePath, nil
}

// InstallGithubReleaseForVersion matches a Github release tag for the
// specified version, then calls InstallGithubReleaseForTag(). The release tag
// is matched from the specified version using
// findGithubReleaseTagForVersion().
func (j JKL) InstallGithubReleaseForVersion(ownerAndRepo, version string) (binaryPath string, err error) {
	ownerAndRepo = strings.Replace(ownerAndRepo, "github.com/", "", 1)
	d := NewDownloader()
	tag, ok, err := d.findGithubReleaseTagForVersion(ownerAndRepo, version)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("no tag found matching version %q", version)
	}
	return j.InstallGithubReleaseForTag(ownerAndRepo, tag)
}

func (j JKL) InstallGithubReleaseForTag(ownerAndRepo, tag string) (binaryPath string, err error) {
	debugLog.Printf("installing Github release %q for tag %q\n", tag, ownerAndRepo)
	d := NewDownloader()
	downloadedFile, err := d.DownloadGithubReleaseForTag(ownerAndRepo, tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	err = ExtractFile(downloadedFile)
	if err != nil {
		return "", err
	}
	toolName := filepath.Base(ownerAndRepo) // name of the repository
	extractedToolBinary := fmt.Sprintf("%s/%s", filepath.Dir(downloadedFile), toolName)
	installDest := fmt.Sprintf("%s/%s/%s", j.installsDir, toolName, tag)
	err = CopyExecutableToCreatedDir(extractedToolBinary, installDest)
	if err != nil {
		return "", err
	}
	binaryPath = fmt.Sprintf("%s/%s", installDest, toolName)
	return binaryPath, nil

}

func (j JKL) InstallGithubReleaseForLatest(ownerAndRepo string) (binaryPath, latestVersionTag string, err error) {
	d := NewDownloader()
	latestVersionTag, err = d.GetGithubLatestReleaseTag(ownerAndRepo)
	if err != nil {
		return "", "", err
	}
	binaryPath, err = j.InstallGithubReleaseForTag(ownerAndRepo, latestVersionTag)
	return binaryPath, latestVersionTag, err
}

func (d Downloader) findGithubReleaseTagForVersion(ownerAndRepo, version string) (tag string, found bool, err error) {
	debugLog.Printf("finding Github tag matching version %q of %q\n", version, ownerAndRepo)
	URI := "/repos/" + ownerAndRepo + "/releases"
	resp, err := d.githubAPIRequest(http.MethodGet, URI)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
	}
	var APIResp GithubReleases
	err = json.NewDecoder(resp.Body).Decode(&APIResp)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if len(APIResp) == 0 {
		return "", false, errors.New("there are no releases")
	}
	tag, found = APIResp.tagExists(version)
	if found {
		return tag, true, nil
	}
	tag, found = APIResp.tagExists(toggleVPrefix(version))
	if found {
		return tag, true, nil
	}
	tag, found = APIResp.tagForReleaseName(version)
	if found {
		return tag, true, nil
	}
	tag, found = APIResp.tagForReleaseName(toggleVPrefix(version))
	if found {
		return tag, true, nil
	}
	tag, found = APIResp.MatchTagFromPartialVersion(version)
	if found {
		return tag, true, nil
	}
	return "", false, nil
}

func (d Downloader) DownloadGithubReleaseForTag(ownerAndRepo, tag, OS, arch string) (filePath string, err error) {
	assets, err := d.ListGithubAssetsForTag(ownerAndRepo, tag)
	if err != nil {
		return "", err
	}
	asset, ok := MatchGithubAsset(assets, OS, arch)
	if !ok {
		return "", fmt.Errorf("no asset found matching Github owner/repository %s, tag %s, OS %s, and architecture %s", ownerAndRepo, tag, OS, arch)
	}
	filePath, err = d.Download(asset)
	return filePath, err
}

func MatchGithubAsset(assets []GithubAsset, OS, arch string) (GithubAsset, bool) {
	archAliases := map[string][]string{
		"amd64": {"x86_64"},
	}
	LCOS := strings.ToLower(OS)
	LCArch := strings.ToLower(arch)
	for _, asset := range assets {
		LCAssetName := strings.ToLower(asset.Name)
		if strings.Contains(LCAssetName, LCOS) && stringContainsOneOf(LCAssetName, LCArch, archAliases[LCArch]...) {
			debugLog.Printf("matched this asset for OS %q and arch %q: %#v", OS, arch, asset)
			return asset, true
		}
	}
	if LCOS == "darwin" && LCArch == "arm64" {
		// If no Darwin/ARM64 asset is available, try AMD64 which can run under Mac OS
		// Rosetta.
		debugLog.Println("trying to match Github asset for Darwin/AMD64 as none were found for ARM64")
		return MatchGithubAsset(assets, OS, "amd64")
	}
	return GithubAsset{}, false
}
