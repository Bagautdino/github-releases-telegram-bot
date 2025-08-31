package db

import (
	"context"
	"database/sql"
	"time"
)

// Repository represents a GitHub repository to track
type Repository struct {
	ID               int    `json:"id"`
	Owner            string `json:"owner"`
	Name             string `json:"name"`
	TrackPrereleases bool   `json:"track_prereleases"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Language string `json:"language"`
}

// ProcessedRelease represents a release that has been processed
type ProcessedRelease struct {
	RepoOwner   string    `json:"repo_owner"`
	RepoName    string    `json:"repo_name"`
	ReleaseID   int64     `json:"release_id"`
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store provides database operations
type Store struct {
	db *DB
}

// NewStore creates a new store instance
func NewStore(db *DB) *Store {
	return &Store{db: db}
}

// Repository operations

// AddRepository adds a new repository to track
func (s *Store) AddRepository(ctx context.Context, owner, name string, trackPrereleases bool) error {
	query := `INSERT OR REPLACE INTO repos (owner, name, track_prereleases) VALUES (?, ?, ?)`
	_, err := s.db.conn.ExecContext(ctx, query, owner, name, boolToInt(trackPrereleases))
	return err
}

// RemoveRepository removes a repository from tracking
func (s *Store) RemoveRepository(ctx context.Context, owner, name string) error {
	query := `DELETE FROM repos WHERE owner = ? AND name = ?`
	_, err := s.db.conn.ExecContext(ctx, query, owner, name)
	return err
}

// ListRepositories returns all tracked repositories
func (s *Store) ListRepositories(ctx context.Context) ([]Repository, error) {
	query := `SELECT id, owner, name, track_prereleases FROM repos ORDER BY owner, name`
	rows, err := s.db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var r Repository
		var trackPrereleases int
		if err := rows.Scan(&r.ID, &r.Owner, &r.Name, &trackPrereleases); err != nil {
			return nil, err
		}
		r.TrackPrereleases = trackPrereleases == 1
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// Chat operations

// AddChat adds a new chat
func (s *Store) AddChat(ctx context.Context, chatID int64, title, language string) error {
	query := `INSERT OR REPLACE INTO chats (id, title, language) VALUES (?, ?, ?)`
	_, err := s.db.conn.ExecContext(ctx, query, chatID, title, language)
	return err
}

// RemoveChat removes a chat
func (s *Store) RemoveChat(ctx context.Context, chatID int64) error {
	query := `DELETE FROM chats WHERE id = ?`
	_, err := s.db.conn.ExecContext(ctx, query, chatID)
	return err
}

// ListChats returns all registered chats
func (s *Store) ListChats(ctx context.Context) ([]Chat, error) {
	query := `SELECT id, title, language FROM chats ORDER BY id`
	rows, err := s.db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.Title, &c.Language); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

// Processed releases operations

// MarkProcessed marks a release as processed
func (s *Store) MarkProcessed(ctx context.Context, repoOwner, repoName string, releaseID int64, tagName string, publishedAt time.Time) error {
	query := `INSERT OR REPLACE INTO processed_releases (repo_owner, repo_name, release_id, tag_name, published_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.conn.ExecContext(ctx, query, repoOwner, repoName, releaseID, tagName, publishedAt.Format(time.RFC3339))
	return err
}

// IsProcessed checks if a release has been processed
func (s *Store) IsProcessed(ctx context.Context, repoOwner, repoName string, releaseID int64) (bool, error) {
	query := `SELECT 1 FROM processed_releases WHERE repo_owner = ? AND repo_name = ? AND release_id = ?`
	var exists int
	err := s.db.conn.QueryRowContext(ctx, query, repoOwner, repoName, releaseID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ETag operations

// GetETag returns the stored ETag for a repository
func (s *Store) GetETag(ctx context.Context, repoOwner, repoName string) (string, error) {
	query := `SELECT etag FROM etags WHERE repo_owner = ? AND repo_name = ?`
	var etag string
	err := s.db.conn.QueryRowContext(ctx, query, repoOwner, repoName).Scan(&etag)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return etag, nil
}

// PutETag stores an ETag for a repository
func (s *Store) PutETag(ctx context.Context, repoOwner, repoName, etag string) error {
	query := `INSERT OR REPLACE INTO etags (repo_owner, repo_name, etag, updated_at) VALUES (?, ?, ?, datetime('now'))`
	_, err := s.db.conn.ExecContext(ctx, query, repoOwner, repoName, etag)
	return err
}

// Settings operations

// GetSetting retrieves a setting value
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM settings WHERE key = ?`
	var value string
	err := s.db.conn.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting stores a setting value
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	query := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`
	_, err := s.db.conn.ExecContext(ctx, query, key, value)
	return err
}

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
