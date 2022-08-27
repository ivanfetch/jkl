package jkl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type GithubClient struct {
	token, apiHost string
	httpClient     *http.Client
}

// githubClientOption specifies GithubClient options as functions.
type githubClientOption func(*GithubClient) error

// WithAPIHost sets the Github API hostname for an instance of GithubClient.
func WithAPIHost(host string) githubClientOption {
	return func(c *GithubClient) error {
		c.apiHost = host
		return nil
	}
}

// WithHTTPClient sets a custom net/http.Client for an instance of GithubClient.
func WithHTTPClient(hc *http.Client) githubClientOption {
	return func(c *GithubClient) error {
		c.httpClient = hc
		return nil
	}
}

func NewGithubClient(options ...githubClientOption) (*GithubClient, error) {
	c := &GithubClient{
		apiHost:    "https://api.github.com",
		token:      os.Getenv("GH_TOKEN"),
		httpClient: &defaultHTTPClient,
	}
	for _, o := range options {
		err := o(c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

type GithubAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

var assetBaseNameRE *regexp.Regexp = regexp.MustCompile(`(?is)^(.+?)[-_]v?\d+.*`)

// GetBaseName returns the asset name after attempting to strip version,
// architecture, operating system, and file extension.
func (g GithubAsset) GetBaseName() string {
	matches := assetBaseNameRE.FindStringSubmatch(g.Name)
	if matches == nil || len(matches) < 2 {
		simplerMatches := strings.FieldsFunc(g.Name, func(r rune) bool {
			return r == '-' || r == '_'
		})
		if len(simplerMatches) == 0 {
			debugLog.Printf("unable to match a base asset name from %q, returning the full asset name", g.Name)
			return g.Name
		}
		debugLog.Printf("matched simpler base name %q for asset name %q", simplerMatches[0], g.Name)
		return simplerMatches[0]
	}
	debugLog.Printf("matched base name %q for asset name %q", matches[1], g.Name)
	return matches[1]
}

type GithubReleases []struct {
	ReleaseName string `json:"name"`
	TagName     string `json:"tag_name"`
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

func (g GithubReleases) MatchTagFromPartialVersion(pv string) (tag string, found bool) {
	debugLog.Printf("matching tag from partial version %q\n", pv)
	tags := make([]string, len(g))
	for i, j := range g {
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

type GithubRepo struct {
	ownerAndRepo string
	client       *GithubClient
}

func NewGithubRepo(ownerAndRepo string, clientOptions ...githubClientOption) (*GithubRepo, error) {
	if ownerAndRepo == "" {
		return nil, errors.New("the repository cannot be empty, please specify a repository of the form OwnerName/RepositoryName")
	}
	if !strings.Contains(ownerAndRepo, "/") {
		return nil, errors.New("the repository must be of the form OwnerName/RepositoryName")
	}
	ownerAndRepo = strings.Replace(ownerAndRepo, "github.com/", "", 1)
	c, err := NewGithubClient(clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("while constructing Github client for repository %s: %w", ownerAndRepo, err)
	}
	return &GithubRepo{
		ownerAndRepo: ownerAndRepo,
		client:       c,
	}, nil
}

func (g GithubRepo) GetOwnerAndRepo() string {
	return g.ownerAndRepo
}

func (g GithubRepo) Exists() (bool, error) {
	URI := "/repos/" + g.ownerAndRepo
	resp, err := g.githubAPIRequest(http.MethodGet, URI)
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

func (g *GithubRepo) githubAPIRequest(method, URI string) (*http.Response, error) {
	if !strings.HasPrefix(URI, "/") {
		URI = "/" + URI
	}
	URL := g.client.apiHost + URI
	req, err := http.NewRequest(method, URL, nil)
	if err != nil {
		return nil, err
	}
	if g.client.token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", g.client.token))
	}
	resp, err := g.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (g GithubRepo) AssetsForTag(tag string) ([]GithubAsset, error) {
	ok, err := g.Exists()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("no such repository")
	}
	URI := "/repos/" + g.ownerAndRepo + "/releases/tags/" + tag
	resp, err := g.githubAPIRequest(http.MethodGet, URI)
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

func (g GithubRepo) GetTagForLatestRelease() (tagName string, err error) {
	ok, err := g.Exists()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("no such repository")
	}
	URI := "/repos/" + g.ownerAndRepo + "/releases/latest"
	resp, err := g.githubAPIRequest(http.MethodGet, URI)
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

func (g GithubRepo) Download(asset GithubAsset) (filePath string, err error) {
	req, err := http.NewRequest(http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	if g.client.token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", g.client.token))
	}
	req.Header.Add("Accept", "application/octet-stream")
	resp, err := g.client.httpClient.Do(req)
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

// DownloadReleaseForVersion matches a Github release tag for the
// specified version, then calls DownloadReleaseForTag().
// The release tag is matched from the specified version using
// findGithubReleaseTagForVersion().
// An empty version causes the latest release to be installed.
func (g GithubRepo) DownloadReleaseForVersion(version string) (binaryPath, matchedTag, assetBaseName string, err error) {
	tag, ok, err := g.findTagForVersion(version)
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", fmt.Errorf("no tag found matching version %q", version)
	}
	binaryPath, assetBaseName, err = g.DownloadReleaseForTag(tag)
	return binaryPath, tag, assetBaseName, err
}

// findTagForVersion matches a release tag to the specified version. An empty
// version or "latest" will return the latest release tag.
func (g GithubRepo) findTagForVersion(version string) (tag string, found bool, err error) {
	debugLog.Printf("finding Github tag matching version %q of %q\n", version, g.GetOwnerAndRepo())
	if version == "" || strings.EqualFold(version, "latest") {
		tag, err = g.GetTagForLatestRelease()
		if err != nil {
			return "", false, err
		}
		return tag, true, nil
	}
	URI := "/repos/" + g.ownerAndRepo + "/releases"
	resp, err := g.githubAPIRequest(http.MethodGet, URI)
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

func (g GithubRepo) DownloadReleaseForLatest() (binaryPath, latestVersionTag, assetBaseName string, err error) {
	latestVersionTag, err = g.GetTagForLatestRelease()
	if err != nil {
		return "", "", "", err
	}
	binaryPath, assetBaseName, err = g.DownloadReleaseForTag(latestVersionTag)
	return binaryPath, latestVersionTag, assetBaseName, err
}

func (g GithubRepo) DownloadReleaseForTagOSAndArch(tag, OS, arch string) (filePath, baseAssetName string, err error) {
	assets, err := g.AssetsForTag(tag)
	if err != nil {
		return "", "", err
	}
	asset, ok := MatchAssetByOsAndArch(assets, OS, arch)
	if !ok {
		return "", "", fmt.Errorf("no asset found matching Github owner/repository %s, tag %s, OS %s, and architecture %s", g.ownerAndRepo, tag, OS, arch)
	}
	filePath, err = g.Download(asset)
	return filePath, asset.GetBaseName(), err
}

func (g GithubRepo) DownloadReleaseForTag(tag string) (binaryPath, assetBaseName string, err error) {
	debugLog.Printf("downloading Github release %q for tag %q\n", tag, g.ownerAndRepo)
	downloadedFile, assetBaseName, err := g.DownloadReleaseForTagOSAndArch(tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", err
	}
	return downloadedFile, assetBaseName, nil
}

func MatchAssetByOsAndArch(assets []GithubAsset, OS, arch string) (GithubAsset, bool) {
	archAliases := map[string][]string{
		"amd64": {"x86_64"},
	}
	OSAliases := map[string][]string{
		"darwin": {"macos"},
	}
	LCOS := strings.ToLower(OS)
	LCArch := strings.ToLower(arch)
	for _, asset := range assets {
		LCAssetName := strings.ToLower(asset.Name)
		if stringContainsOneOf(LCAssetName, LCOS, OSAliases[LCOS]...) && stringContainsOneOf(LCAssetName, LCArch, archAliases[LCArch]...) {
			debugLog.Printf("matched this asset for OS %q and arch %q: %#v", OS, arch, asset)
			return asset, true
		}
	}
	if LCOS == "darwin" && LCArch == "arm64" {
		// If no Darwin/ARM64 asset is available, try AMD64 which can run under Mac OS
		// Rosetta.
		debugLog.Println("trying to match Github asset for Darwin/AMD64 as none were found for ARM64")
		return MatchAssetByOsAndArch(assets, OS, "amd64")
	}
	return GithubAsset{}, false
}
