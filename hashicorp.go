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
)

type HashicorpClient struct {
	httpClient *http.Client
	apiHost    string
}

// hashicorpClientOption specifies HashicorpClient options as functions.
type hashicorpClientOption func(*HashicorpClient) error

// WithHashicorpAPIHost sets the Hashicorp API hostname for an instance of HashicorpClient.
func WithHashicorpAPIHost(host string) hashicorpClientOption {
	return func(c *HashicorpClient) error {
		if host == "" {
			return errors.New("the API host cannot be empty")
		}
		c.apiHost = host
		return nil
	}
}

// WithHashicorpHTTPClient sets a custom net/http.Client for an instance of HashicorpClient.
func WithHashicorpHTTPClient(hc *http.Client) hashicorpClientOption {
	return func(c *HashicorpClient) error {
		if hc == nil {
			return errors.New("the HTTP client cannot be nil")
		}
		c.httpClient = hc
		return nil
	}
}

func NewHashicorpClient(options ...hashicorpClientOption) (*HashicorpClient, error) {
	c := &HashicorpClient{
		apiHost:    "https://api.releases.hashicorp.com",
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

type hashicorpBuild struct {
	Arch string `json:"arch"`
	OS   string `json:"os"`
	URL  string `json:"url"`
}

type hashicorpRelease struct {
	Version          string           `json:"version"`
	Builds           []hashicorpBuild `json:"builds"`
	TimestampCreated string           `json:"timestamp_created"` // needed for API pagination
	IsPrerelease     bool             `json:"is_prerelease"`
}

type hashicorpReleases []hashicorpRelease

func (r hashicorpReleases) forPartialVersion(pv string) (release hashicorpRelease, found bool) {
	if len(r) == 0 {
		debugLog.Printf("cannot match a partial version %q from 0 Hashicorp releases", pv)
		return hashicorpRelease{}, false
	}
	debugLog.Printf("matching version from partial version %q in %d Hashicorp releases", pv, len(r))
	releasesByVersion := make(map[string]hashicorpRelease, len(r))
	var partialMatches []string
	LCPV := strings.ToLower(pv)
	for _, j := range r {
		releasesByVersion[j.Version] = j
		if j.IsPrerelease {
			debugLog.Printf("skipping pre-release %q\n", j.Version)
			continue
		}
		LCThisVersion := strings.ToLower(j.Version)
		if strings.HasPrefix(LCThisVersion, LCPV) || strings.HasPrefix(LCThisVersion, "v"+LCPV) {
			debugLog.Printf("%q is a partial match", j.Version)
			partialMatches = append(partialMatches, j.Version)
		}
	}
	if len(partialMatches) == 0 {
		debugLog.Printf("no partial matches for version %s\n", pv)
		return hashicorpRelease{}, false
	}
	sort.Strings(partialMatches)
	bestMatch := partialMatches[len(partialMatches)-1]
	debugLog.Printf("matched version %q for partial version %s\n", bestMatch, pv)
	return releasesByVersion[bestMatch], true
}

type HashicorpProduct struct {
	name                       string
	oldestSeenReleaseTimestamp string // pagination marker
	client                     *HashicorpClient
}

func NewHashicorpProduct(name string, clientOptions ...hashicorpClientOption) (*HashicorpProduct, error) {
	if name == "" {
		return nil, errors.New("the product name cannot be empty")
	}
	c, err := NewHashicorpClient(clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("while constructing Hashicorp client for product %s: %w", name, err)
	}
	return &HashicorpProduct{
		name:   name,
		client: c,
	}, nil
}

func (h HashicorpProduct) GetName() string {
	return h.name
}

func (h *HashicorpProduct) hashicorpAPIRequest(method, URI string) (*http.Response, error) {
	if !strings.HasPrefix(URI, "/") {
		URI = "/" + URI
	}
	URL := h.client.apiHost + URI
	req, err := http.NewRequest(method, URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (h HashicorpProduct) Exists() (bool, error) {
	URI := "/v1/products"
	resp, err := h.hashicorpAPIRequest(http.MethodGet, URI)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
	}
	var APIResp []string
	err = json.NewDecoder(resp.Body).Decode(&APIResp)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if len(APIResp) == 0 {
		return false, errors.New("the Hashicorp API did not return any products")
	}
	for _, currentProduct := range APIResp {
		if strings.EqualFold(h.name, currentProduct) {
			return true, nil
		}
	}
	return false, nil
}

func (h *HashicorpProduct) fetchReleases() (hashicorpReleases, error) {
	URI := "/v1/releases/" + h.name + "?limit=20"
	if h.oldestSeenReleaseTimestamp != "" {
		URI += "&after=" + h.oldestSeenReleaseTimestamp
	}
	debugLog.Printf("fetching Hashicorp %s releases with URI %s", h.name, URI)
	resp, err := h.hashicorpAPIRequest(http.MethodGet, URI)
	if err != nil {
		return hashicorpReleases{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return hashicorpReleases{}, fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
	}
	var APIResp hashicorpReleases
	err = json.NewDecoder(resp.Body).Decode(&APIResp)
	if err != nil {
		return hashicorpReleases{}, err
	}
	defer resp.Body.Close()
	debugLog.Printf("fetched %d releases", len(APIResp))
	if len(APIResp) == 0 {
		return hashicorpReleases{}, nil
	}
	oldestTimestamp := APIResp[len(APIResp)-1].TimestampCreated // The API pre-sorts releases
	debugLog.Printf("the oldest release is %q", oldestTimestamp)
	if oldestTimestamp != "" {
		debugLog.Printf("updating oldest seen release timestamp for %s from %q to %q", h.name, h.oldestSeenReleaseTimestamp, oldestTimestamp)
		h.oldestSeenReleaseTimestamp = oldestTimestamp
	}
	return APIResp, nil
}

// releaseForVersion fetches the specified release version, or the latest one
// if an empty string or `latest` is specified.
// IF the explicit version is not found,
// HashicorpProduct.releaseForPartialVersion is called.
func (h HashicorpProduct) releaseForVersion(version string) (release hashicorpRelease, found bool, err error) {
	debugLog.Printf("getting Hashicorp %s release for version %q", h.name, version)
	ok, err := h.Exists()
	if err != nil {
		return hashicorpRelease{}, false, err
	}
	if !ok {
		return hashicorpRelease{}, false, errors.New("no such Hashicorp product")
	}
	if version == "" || strings.EqualFold(version, "latest") {
		version = "latest"
	}
	URI := "/v1/releases/" + h.name + "/" + version
	resp, err := h.hashicorpAPIRequest(http.MethodGet, URI)
	if err != nil {
		return hashicorpRelease{}, false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		debugLog.Printf("Hashicorp %s version %q not found", h.name, version)
		return h.releaseForPartialVersion(version)
	}
	if resp.StatusCode != http.StatusOK {
		return hashicorpRelease{}, false, fmt.Errorf("HTTP %d for %s", resp.StatusCode, URI)
	}
	var APIResp hashicorpRelease
	err = json.NewDecoder(resp.Body).Decode(&APIResp)
	if err != nil {
		return hashicorpRelease{}, false, err
	}
	defer resp.Body.Close()
	if len(APIResp.Builds) == 0 || APIResp.Version == "" {
		debugLog.Printf("received incomplete Hashicorp release %#v", APIResp)
		return hashicorpRelease{}, false, errors.New("the Hashicorp API did not return the expected fields")
	}
	return APIResp, true, nil
}

// releaseForPartialVersion fetches Hashicorp releases, and
// wraps hashicorpReleases.ForPartialVersion until the latest partial version
// is matched, or there are no more releases available.
func (h HashicorpProduct) releaseForPartialVersion(version string) (release hashicorpRelease, found bool, err error) {
	debugLog.Printf("finding Hashicorp %s release matching partial version %q", h.name, version)
	if version == "" || strings.EqualFold(version, "latest") {
		return h.releaseForVersion("latest")
	}
	var releases hashicorpReleases
	releases, err = h.fetchReleases()
	if err != nil {
		return hashicorpRelease{}, false, err
	}
	if len(releases) == 0 {
		return hashicorpRelease{}, false, errors.New("there are no releases")
	}
	for len(releases) > 0 {
		var release hashicorpRelease
		release, found := releases.forPartialVersion(version)
		if found {
			return release, true, nil
		}
		releases, err = h.fetchReleases()
		if err != nil {
			return hashicorpRelease{}, false, err
		}
	}
	debugLog.Println("no partial releases matched")
	return hashicorpRelease{}, false, nil
}

func (h HashicorpProduct) Download(build hashicorpBuild) (filePath string, err error) {
	debugLog.Printf("downloading Hashicorp build from %s", build.URL)
	req, err := http.NewRequest(http.MethodGet, build.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/octet-stream")
	resp, err := h.client.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, build.URL)
	}
	tempDir, err := os.MkdirTemp(os.TempDir(), callMeProgName+"-")
	if err != nil {
		return "", err
	}
	filePath = fmt.Sprintf("%s/%s", tempDir, filepath.Base(build.URL))
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

// DownloadReleaseForVersion downloads the specified version of the Hashicorp
// product, returning the path to the downloaded file, and the version that
// was downloaded.
// A version of `latest` or an empty string will download the latest
// non-pre-release version.
func (h HashicorpProduct) DownloadReleaseForVersion(version string) (binaryPath, matchedVersion string, err error) {
	release, ok, err := h.releaseForVersion(version)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("no version found to match %q", version)
	}
	debugLog.Printf("downloading Hashicorp release for %s version %q\n", h.name, release.Version)
	build, ok := MatchBuildByOsAndArch(release.Builds, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return "", "", fmt.Errorf("no builds of %s version %s match OS %q and architecture %q", h.name, version, runtime.GOOS, runtime.GOARCH)
	}
	downloadedFile, err := h.Download(build)
	if err != nil {
		return "", "", err
	}
	return downloadedFile, release.Version, nil
}

func MatchBuildByOsAndArch(builds []hashicorpBuild, OS, arch string) (hashicorpBuild, bool) {
	debugLog.Printf("matching Hashicorp build by OS %q and architecture %q", OS, arch)
	LCOS := strings.ToLower(OS)
	LCArch := strings.ToLower(arch)
	for _, build := range builds {
		LCBuildArch := strings.ToLower(build.Arch)
		LCBuildOS := strings.ToLower(build.OS)
		if stringEqualFoldOneOf(LCBuildOS, LCOS, getAliasesForOperatingSystem(LCOS)...) && stringEqualFoldOneOf(LCBuildArch, LCArch, getAliasesForArchitecture(LCArch)...) {
			debugLog.Printf("matched this asset for OS %q and arch %q: %#v", OS, arch, build)
			return build, true
		}
	}
	if LCOS == "darwin" && LCArch == "arm64" {
		// If no Darwin/ARM64 asset is available, try AMD64 which can run under Mac OS
		// Rosetta.
		debugLog.Println("trying to match Hashicorp build for Darwin/AMD64 as none were found for ARM64")
		return MatchBuildByOsAndArch(builds, OS, "amd64")
	}
	debugLog.Printf("no Hashicorp build matched OS %s and architecture %s", OS, arch)
	return hashicorpBuild{}, false
}
