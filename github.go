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

// GithubDownload accepts a type toolSpec and populates it with the path of the
// downloaded file and the name of the tool, as
// determined by its assets. The toolSpec may also be updated with the
// version of the tool that was downloaded, in cases where a partial or
// "latest" version is specified.
func GithubDownload(TS *ToolSpec) error {
	g, err := NewGithubRepo(TS.source)
	if err != nil {
		return err
	}
	downloadPath, downloadVersion, downloadName, err := g.DownloadReleaseForVersion(TS.version)
	if err != nil {
		return err
	}
	TS.name = downloadName
	TS.version = downloadVersion
	TS.downloadPath = downloadPath
	return nil
}

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

var versionRE *regexp.Regexp = regexp.MustCompile(`(.+?)[-_]?v?\d+.*`)

// NameWithoutVersionAndComponents returns the asset name minus its version
// and any specified components. A component is matched with a preseeding
// underscore (_) or dash (-). For example, specifying a component of "darwin"
// would strip "-darwin" and "_darwin".
func (g GithubAsset) NameWithoutVersionAndComponents(components ...string) string {
	debugLog.Printf("stripping the asset name %s of its version and components %v", g.Name, components)
	var strippedName = g.Name
	for _, component := range components {
		strippedName = strings.Replace(strippedName, fmt.Sprintf("-%s", component), "", -1)
		strippedName = strings.Replace(strippedName, fmt.Sprintf("_%s", component), "", -1)
	}
	// Attempt to strip what looks like a version, which may still be included in
	// the name.
	withoutVersionMatches := versionRE.FindStringSubmatch(strippedName)
	if withoutVersionMatches != nil || len(withoutVersionMatches) >= 2 {
		debugLog.Printf("the stripped name after matching a version number is %q", withoutVersionMatches[1])
		return withoutVersionMatches[1]
	}
	debugLog.Printf("the stripped name is %q", strippedName)
	return strippedName
}

type GithubReleases []struct {
	ReleaseName string `json:"name"`
	TagName     string `json:"tag_name"`
	PreRelease  bool   `json:"prerelease"`
}

// tagForReleaseName returns the tag for the specified release name. The
// release name and its tag are often identical, but not always...
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

// MatchTagFromPartialVersion returns a latest tag matching an imcomplete
// version E.G. return the latest tag x.y.z for a specified x.y, or x.
func (g GithubReleases) MatchTagFromPartialVersion(pv string) (tag string, found bool) {
	debugLog.Printf("matching tag from partial version %q\n", pv)
	tags := make([]string, len(g))
	for i, j := range g {
		if !j.PreRelease {
			tags[i] = j.TagName
		}
	}
	sort.Strings(tags)
	LCPV := strings.ToLower(pv)
	// Iterate the Github release tags backwards.
	for i := len(tags) - 1; i >= 0; i-- {
		LCThisTag := strings.ToLower(tags[i])
		if strings.HasPrefix(LCThisTag, LCPV) || strings.HasPrefix(LCThisTag, "v"+LCPV) {
			debugLog.Printf("matched tag %q for partial version %s\n", tags[i], pv)
			return tags[i], true
		}
	}
	// Try matching with extraneous text removed from the beginning of the tag,
	// like tags that include the repo or release name.
	var stripPrefixRE *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z-_]+(v?\d+\..*)`)
	for i := len(tags) - 1; i >= 0; i-- {
		LCThisTag := strings.ToLower(tags[i])
		strippedMatches := stripPrefixRE.FindStringSubmatch(LCThisTag)
		if strippedMatches == nil || len(strippedMatches) < 2 {
			debugLog.Printf("cannot strip extraneous text from tag %q\n", LCThisTag)
			continue
		}
		strippedTag := strippedMatches[1]
		debugLog.Printf("the stripped tag is %q", strippedTag)
		if strings.HasPrefix(strippedTag, LCPV) || strings.HasPrefix(strippedTag, "v"+LCPV) {
			debugLog.Printf("matched tag %q after stripping prefix %q, for partial version %s\n", tags[i], strippedTag, pv)
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
	asset, matchedOS, matchedArch, ok := MatchAssetByOsAndArch(assets, OS, arch)
	if !ok {
		return "", "", fmt.Errorf("no asset found matching Github owner/repository %s, tag %s, OS %s, and architecture %s", g.ownerAndRepo, tag, OS, arch)
	}
	filePath, err = g.Download(asset)
	if err != nil {
		return "", "", err
	}
	return filePath, asset.NameWithoutVersionAndComponents(matchedOS, matchedArch, tag), nil
}

func (g GithubRepo) DownloadReleaseForTag(tag string) (binaryPath, assetBaseName string, err error) {
	debugLog.Printf("downloading Github release %q for tag %q\n", g.ownerAndRepo, tag)
	downloadedFile, assetBaseName, err := g.DownloadReleaseForTagOSAndArch(tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", err
	}
	return downloadedFile, assetBaseName, nil
}

func MatchAssetByOsAndArch(assets []GithubAsset, OS, arch string) (matchedAsset GithubAsset, matchedOS, matchedArch string, successfulMatch bool) {
	for _, asset := range assets {
		matchedOS, foundOS := stringContainsOneOfLowerCase(asset.Name, OS, getAliasesForOperatingSystem(OS)...)
		matchedArch, foundArch := stringContainsOneOfLowerCase(asset.Name, arch, getAliasesForArchitecture(arch)...)
		if foundOS && foundArch {
			debugLog.Printf("matched this asset for OS %q and arch %q: %#v", OS, arch, asset)
			return asset, matchedOS, matchedArch, true
		}
		if strings.EqualFold(OS, "linux") && strings.EqualFold(arch, "amd64") && strings.Contains(strings.ToLower(asset.Name), "linux64") {
			debugLog.Printf("matched this asset against the combo-string linux64: %#v\n", asset)
			return asset, "linux64", "linux64", true // OS and arch are linux64 to facilitate stripping components from the asset name
		}
	}
	if strings.EqualFold(OS, "darwin") && strings.EqualFold(arch, "arm64") {
		// If no Darwin/ARM64 asset is available, try AMD64 which can run under Mac OS
		// Rosetta.
		debugLog.Println("trying to match Github asset for Darwin/AMD64 as none were found for ARM64")
		return MatchAssetByOsAndArch(assets, OS, "amd64")
	}
	return GithubAsset{}, "", "", false
}
