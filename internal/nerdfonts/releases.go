package nerdfonts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	releasesURL            = "https://api.github.com/repos/ryanoasis/nerd-fonts/releases"
	defaultMaxReleasePages = 5
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	MaxPages   int
}

type Release struct {
	Name     string
	TagName  string
	Families []string
}

func (c Client) Releases(ctx context.Context) ([]Release, error) {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = releasesURL
	}
	maxPages := c.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxReleasePages
	}

	releases := []Release{}
	for page := 1; page <= maxPages; page++ {
		pageURL, err := withPage(baseURL, page)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create releases request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "nerdfont-install")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list Nerd Fonts releases: %w", err)
		}

		pageReleases, err := decodeReleases(resp)
		if err != nil {
			return nil, err
		}
		if len(pageReleases) == 0 {
			break
		}

		releases = append(releases, pageReleases...)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no Nerd Fonts releases found")
	}
	return releases, nil
}

func decodeReleases(resp *http.Response) ([]Release, error) {
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("list Nerd Fonts releases: %s", resp.Status)
	}

	var apiReleases []struct {
		Name    string `json:"name"`
		TagName string `json:"tag_name"`
		Draft   bool   `json:"draft"`
		Assets  []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiReleases); err != nil {
		return nil, fmt.Errorf("decode Nerd Fonts releases: %w", err)
	}

	releases := make([]Release, 0, len(apiReleases))
	for _, apiRelease := range apiReleases {
		if apiRelease.Draft || strings.TrimSpace(apiRelease.TagName) == "" {
			continue
		}

		assetNames := make([]string, 0, len(apiRelease.Assets))
		for _, asset := range apiRelease.Assets {
			assetNames = append(assetNames, asset.Name)
		}

		families := familiesFromAssets(assetNames)
		if len(families) == 0 {
			continue
		}

		name := strings.TrimSpace(apiRelease.Name)
		if name == "" {
			name = apiRelease.TagName
		}
		releases = append(releases, Release{
			Name:     name,
			TagName:  apiRelease.TagName,
			Families: families,
		})
	}
	return releases, nil
}

func familiesFromAssets(assets []string) []string {
	seen := map[string]bool{}
	families := []string{}
	for _, name := range assets {
		if !strings.EqualFold(path.Ext(name), ".zip") {
			continue
		}
		family := strings.TrimSuffix(name, path.Ext(name))
		if family == "" || seen[family] {
			continue
		}
		seen[family] = true
		families = append(families, family)
	}
	sort.Strings(families)
	return families
}

func withPage(rawURL string, page int) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse releases url: %w", err)
	}
	query := parsed.Query()
	query.Set("per_page", "100")
	query.Set("page", fmt.Sprint(page))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
