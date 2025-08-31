package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/tg-release-bot/internal/advisor"
	"github.com/yourorg/tg-release-bot/internal/compose"
	"github.com/yourorg/tg-release-bot/internal/config"
	"github.com/yourorg/tg-release-bot/internal/db"
	"github.com/yourorg/tg-release-bot/internal/github"
	"github.com/yourorg/tg-release-bot/internal/logging"
	"github.com/yourorg/tg-release-bot/internal/scheduler"
	"github.com/yourorg/tg-release-bot/internal/telegram"
)

func main() {
	// Setup logging
	logger := logging.Setup()
	logger.Info("Starting Telegram Release Bot")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		logger.Info("Received shutdown signal")
		cancel()
	}()

	// Initialize database
	dbPath := getEnv("DB_PATH", "./releases.db")
	database, err := db.Open(dbPath)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	store := db.NewStore(database)

	// Initialize GitHub client
	githubClient := github.New(cfg.GithubToken)

	// Initialize Telegram sender
	telegramSender, err := telegram.NewSender(cfg.TelegramToken)
	if err != nil {
		logger.Error("Failed to create Telegram sender", "error", err)
		os.Exit(1)
	}

	// Initialize LLM advisor (optional)
	var advisorClient *advisor.Client
	if cfg.AdvisorEnabled {
		advisorClient = advisor.New(cfg.OpenRouterAPIKey, cfg.OpenRouterModel)
		logger.Info("LLM advisor enabled", "model", cfg.OpenRouterModel)
	}

	// Create the main job and start scheduler
	job := createReleaseCheckJob(logger, store, githubClient, telegramSender, advisorClient, cfg)
	interval := time.Duration(cfg.IntervalMinutes) * time.Minute
	releaseScheduler := scheduler.New(logger, interval, job)
	releaseScheduler.Start(ctx)

	// Initialize bot for commands (optional)
	var botCommands *telegram.Bot
	if len(cfg.AllowedUserIDs) > 0 {
		storeAdapter := telegram.NewStoreAdapter(store)
		botCommands, err = telegram.NewBot(cfg.TelegramToken, storeAdapter, releaseScheduler, advisorClient, cfg.AllowedUserIDs, logger)
		if err != nil {
			logger.Error("Failed to create bot", "error", err)
		} else {
			logger.Info("Bot commands enabled", "allowed_users", cfg.AllowedUserIDs)
			go botCommands.StartPolling(ctx)
		}
	}

	// Add default chat if specified
	if cfg.DefaultChatID != 0 {
		err = store.AddChat(ctx, cfg.DefaultChatID, "Default Chat", "ru")
		if err != nil {
			logger.Warn("Failed to add default chat", "chat_id", cfg.DefaultChatID, "error", err)
		}
	}

	logger.Info("Bot started successfully",
		"interval", interval,
		"advisor_enabled", cfg.AdvisorEnabled,
		"commands_enabled", len(cfg.AllowedUserIDs) > 0)

	// Wait for shutdown
	<-ctx.Done()
	logger.Info("Shutting down...")

	releaseScheduler.Stop()
	logger.Info("Bot stopped")
}

// createReleaseCheckJob creates the main job function for checking releases
func createReleaseCheckJob(
	logger *slog.Logger,
	store *db.Store,
	githubClient *github.Client,
	telegramSender *telegram.Sender,
	advisorClient *advisor.Client,
	cfg *config.Config,
) scheduler.Job {
	return func(ctx context.Context) {
		logger.Info("Starting release check job")

		// Get all tracked repositories
		repos, err := store.ListRepositories(ctx)
		if err != nil {
			logger.Error("Failed to get repositories", "error", err)
			return
		}

		if len(repos) == 0 {
			logger.Info("No repositories to check")
			return
		}

		logger.Info("Checking releases for repositories", "count", len(repos))

		// Process repositories in batches to avoid rate limiting
		batchSize := 20
		for i := 0; i < len(repos); i += batchSize {
			end := i + batchSize
			if end > len(repos) {
				end = len(repos)
			}

			batch := repos[i:end]
			processBatch(ctx, logger, store, githubClient, telegramSender, advisorClient, cfg, batch)

			// Small delay between batches
			if end < len(repos) {
				time.Sleep(1 * time.Second)
			}
		}

		logger.Info("Release check job completed")
	}
}

// processBatch processes a batch of repositories
func processBatch(
	ctx context.Context,
	logger *slog.Logger,
	store *db.Store,
	githubClient *github.Client,
	telegramSender *telegram.Sender,
	advisorClient *advisor.Client,
	cfg *config.Config,
	repos []db.Repository,
) {
	for i, repo := range repos {
		// Add small delay between requests to be nice to GitHub API
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		processRepository(ctx, logger, store, githubClient, telegramSender, advisorClient, cfg, repo)
	}
}

// processRepository processes a single repository
func processRepository(
	ctx context.Context,
	logger *slog.Logger,
	store *db.Store,
	githubClient *github.Client,
	telegramSender *telegram.Sender,
	advisorClient *advisor.Client,
	cfg *config.Config,
	repo db.Repository,
) {
	repoName := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
	logger = logger.With("repo", repoName)

	// Get stored ETag
	etag, err := store.GetETag(ctx, repo.Owner, repo.Name)
	if err != nil {
		logger.Warn("Failed to get ETag", "error", err)
	}

	// Fetch releases from GitHub
	resp, err := githubClient.ListReleases(ctx, repo.Owner, repo.Name, etag)
	if err != nil {
		logger.Error("Failed to fetch releases", "error", err)
		return
	}

	// Handle 304 Not Modified
	if resp.StatusCode == 304 {
		logger.Debug("No new releases (304 Not Modified)")
		return
	}

	// Update ETag
	if resp.ETag != "" {
		if err := store.PutETag(ctx, repo.Owner, repo.Name, resp.ETag); err != nil {
			logger.Warn("Failed to store ETag", "error", err)
		}
	}

	// Filter and sort releases
	releases := githubClient.FilterAndSortReleases(resp.Releases, repo.TrackPrereleases)

	logger.Debug("Processed releases", "total", len(resp.Releases), "filtered", len(releases))

	// Process each release
	for _, release := range releases {
		processRelease(ctx, logger, store, telegramSender, advisorClient, cfg, repo, release)
	}
}

// processRelease processes a single release
func processRelease(
	ctx context.Context,
	logger *slog.Logger,
	store *db.Store,
	telegramSender *telegram.Sender,
	advisorClient *advisor.Client,
	cfg *config.Config,
	repo db.Repository,
	release github.Release,
) {
	releaseLogger := logger.With("release_id", release.ID, "tag", release.TagName)

	// Check if already processed
	processed, err := store.IsProcessed(ctx, repo.Owner, repo.Name, release.ID)
	if err != nil {
		releaseLogger.Error("Failed to check if release is processed", "error", err)
		return
	}

	if processed {
		releaseLogger.Debug("Release already processed, skipping")
		return
	}

	releaseLogger.Info("Processing new release")

	// Extract bullets from changelog
	bullets := compose.TakeBullets(release.Body, cfg.MaxBullets, cfg.MaxChangelogChars)

	// Get LLM advice if enabled
	var advice string
	if advisorClient != nil {
		repoName := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
		advice, err = advisorClient.Advise(ctx, repoName, release.TagName, bullets)
		if err != nil {
			releaseLogger.Warn("Failed to get LLM advice", "error", err)
			// Continue without advice - don't fail the whole process
		}
	}

	// Compose message
	msg := compose.BuildHTML(compose.Input{
		RepoFull:  fmt.Sprintf("%s/%s", repo.Owner, repo.Name),
		Tag:       release.TagName,
		URL:       release.HTMLURL,
		BodyMD:    release.Body,
		Published: release.PublishedAt,
		Advisor:   advice,
	}, compose.Options{
		MaxBullets: cfg.MaxBullets,
		MaxChars:   cfg.MaxChangelogChars,
		TimeZone:   cfg.TimeZone,
	})

	// Get all chats
	chats, err := store.ListChats(ctx)
	if err != nil {
		releaseLogger.Error("Failed to get chats", "error", err)
		return
	}

	if len(chats) == 0 {
		releaseLogger.Warn("No chats configured for notifications")
		// Still mark as processed to avoid reprocessing
	} else {
		// Send to all chats
		for _, chat := range chats {
			chatLogger := releaseLogger.With("chat_id", chat.ID)

			if err := telegramSender.SendHTML(ctx, chat.ID, msg); err != nil {
				chatLogger.Error("Failed to send message", "error", err)
			} else {
				chatLogger.Info("Message sent successfully")
			}

			// Small delay between messages to different chats
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Mark as processed
	if err := store.MarkProcessed(ctx, repo.Owner, repo.Name, release.ID, release.TagName, release.PublishedAt); err != nil {
		releaseLogger.Error("Failed to mark release as processed", "error", err)
	} else {
		releaseLogger.Info("Release marked as processed")
	}
}

// getEnv returns environment variable or default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
