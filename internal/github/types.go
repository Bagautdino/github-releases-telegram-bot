package github

import "time"

// Release represents a GitHub release
type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// ReleasesResponse represents the response from GitHub API
type ReleasesResponse struct {
	StatusCode int
	ETag       string
	Releases   []Release
}
