package telegram

import (
	"context"

	"github.com/yourorg/tg-release-bot/internal/db"
)

// StoreAdapter adapts db.Store to telegram.Store interface
type StoreAdapter struct {
	store *db.Store
}

// NewStoreAdapter creates a new store adapter
func NewStoreAdapter(store *db.Store) Store {
	return &StoreAdapter{store: store}
}

// AddRepository implements Store.AddRepository
func (a *StoreAdapter) AddRepository(ctx context.Context, owner, name string, trackPrereleases bool) error {
	return a.store.AddRepository(ctx, owner, name, trackPrereleases)
}

// RemoveRepository implements Store.RemoveRepository
func (a *StoreAdapter) RemoveRepository(ctx context.Context, owner, name string) error {
	return a.store.RemoveRepository(ctx, owner, name)
}

// ListRepositories implements Store.ListRepositories
func (a *StoreAdapter) ListRepositories(ctx context.Context) ([]Repository, error) {
	dbRepos, err := a.store.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}

	var repos []Repository
	for _, dbRepo := range dbRepos {
		repos = append(repos, Repository{
			Owner:            dbRepo.Owner,
			Name:             dbRepo.Name,
			TrackPrereleases: dbRepo.TrackPrereleases,
		})
	}
	return repos, nil
}

// AddChat implements Store.AddChat
func (a *StoreAdapter) AddChat(ctx context.Context, chatID int64, title, language string) error {
	return a.store.AddChat(ctx, chatID, title, language)
}

// RemoveChat implements Store.RemoveChat
func (a *StoreAdapter) RemoveChat(ctx context.Context, chatID int64) error {
	return a.store.RemoveChat(ctx, chatID)
}

// ListChats implements Store.ListChats
func (a *StoreAdapter) ListChats(ctx context.Context) ([]Chat, error) {
	dbChats, err := a.store.ListChats(ctx)
	if err != nil {
		return nil, err
	}

	var chats []Chat
	for _, dbChat := range dbChats {
		chats = append(chats, Chat{
			ID:       dbChat.ID,
			Title:    dbChat.Title,
			Language: dbChat.Language,
		})
	}
	return chats, nil
}
