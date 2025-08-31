package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// Client provides GitHub API functionality
type Client struct {
	http      *http.Client
	baseURL   string
	token     string
	userAgent string
}

// New creates a new GitHub client
func New(token string) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:   "https://api.github.com",
		token:     token,
		userAgent: "tg-release-bot/1.0",
	}
}

// ListReleases fetches releases for a repository with ETag support
func (c *Client) ListReleases(ctx context.Context, owner, repo, etag string) (*ReleasesResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=5", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	// Make request with retry logic
	resp, err := c.doWithRetry(req, 3)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	response := &ReleasesResponse{
		StatusCode: resp.StatusCode,
		ETag:       resp.Header.Get("ETag"),
	}

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		return response, nil
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %d %s", resp.StatusCode, string(body))
	}

	// Decode response
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response.Releases = releases
	return response, nil
}

// FilterAndSortReleases filters drafts and sorts releases by published date
func (c *Client) FilterAndSortReleases(releases []Release, trackPrereleases bool) []Release {
	var filtered []Release

	for _, release := range releases {
		// Skip drafts
		if release.Draft {
			continue
		}

		// Skip prereleases if not tracking them
		if release.Prerelease && !trackPrereleases {
			continue
		}

		// Skip releases without published date
		if release.PublishedAt.IsZero() {
			continue
		}

		filtered = append(filtered, release)
	}

	// Sort by published date (ascending - oldest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].PublishedAt.Before(filtered[j].PublishedAt)
	})

	return filtered
}

// doWithRetry performs HTTP request with retry logic for 429 and 5xx errors
func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			break
		}

		// Success or client error (don't retry)
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}

		// Server error or rate limit - retry
		resp.Body.Close()
		if attempt < maxRetries {
			backoff := time.Duration(attempt+1) * time.Second
			if resp.StatusCode == 429 {
				// For rate limiting, use longer backoff
				backoff = time.Duration(attempt+1) * 2 * time.Second
			}
			time.Sleep(backoff)
		}
		lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
	}

	return nil, lastErr
}
