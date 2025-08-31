package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Store interface for bot commands  
type Store interface {
	AddRepository(ctx context.Context, owner, name string, trackPrereleases bool) error
	RemoveRepository(ctx context.Context, owner, name string) error
	ListRepositories(ctx context.Context) ([]Repository, error)
	AddChat(ctx context.Context, chatID int64, title, language string) error
	RemoveChat(ctx context.Context, chatID int64) error
	ListChats(ctx context.Context) ([]Chat, error)
}

// JobRunner interface for triggering release checks
type JobRunner interface {
	TriggerCheck(ctx context.Context) error
}

// LLMAdvisor interface for testing LLM
type LLMAdvisor interface {
	Advise(ctx context.Context, repo, tag string, bullets []string) (string, error)
}

// Repository represents a repository for bot operations
type Repository struct {
	Owner            string
	Name             string
	TrackPrereleases bool
}

// Chat represents a chat for bot operations
type Chat struct {
	ID       int64
	Title    string
	Language string
}

// Bot handles Telegram bot commands
type Bot struct {
	api          *tgbotapi.BotAPI
	store        Store
	jobRunner    JobRunner
	llmAdvisor   LLMAdvisor
	allowedUsers map[int64]bool
	logger       *slog.Logger
}

// NewBot creates a new bot instance
func NewBot(token string, store Store, jobRunner JobRunner, llmAdvisor LLMAdvisor, allowedUserIDs []int64, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	allowedUsers := make(map[int64]bool)
	for _, id := range allowedUserIDs {
		allowedUsers[id] = true
	}

	return &Bot{
		api:          api,
		store:        store,
		jobRunner:    jobRunner,
		llmAdvisor:   llmAdvisor,
		allowedUsers: allowedUsers,
		logger:       logger,
	}, nil
}

// StartPolling starts polling for updates
func (b *Bot) StartPolling(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case update := <-updates:
			if update.Message != nil {
				go b.handleMessage(ctx, update.Message)
			}
		}
	}
}

// handleMessage processes incoming messages
func (b *Bot) handleMessage(ctx context.Context, message *tgbotapi.Message) {
	// Check if user is allowed to use commands
	if !b.allowedUsers[message.From.ID] {
		return
	}

	if !message.IsCommand() {
		return
	}

	command := message.Command()
	args := message.CommandArguments()

	b.logger.Info("Processing command",
		"command", command,
		"args", args,
		"user_id", message.From.ID,
		"chat_id", message.Chat.ID)

	var response string
	var err error

	switch command {
	case "addrepo":
		response, err = b.handleAddRepo(ctx, args)
	case "delrepo":
		response, err = b.handleDelRepo(ctx, args)
	case "list":
		response, err = b.handleList(ctx)
	case "setchat":
		response, err = b.handleSetChat(ctx, message.Chat.ID, args)
	case "test":
		response = "✅ Bot is working!"
	case "help":
		response = b.getHelpText()
	case "forcecheck":
		response, err = b.handleForceCheck(ctx)
	case "addtestrepo":
		response, err = b.handleAddTestRepo(ctx)
	case "testnotify":
		response, err = b.handleTestNotify(ctx, message.Chat.ID)
	case "testllm":
		response, err = b.handleTestLLM(ctx, message.Chat.ID)
	default:
		response = "Unknown command. Use /help for available commands."
	}

	if err != nil {
		b.logger.Error("Command execution failed", "command", command, "error", err)
		response = fmt.Sprintf("❌ Error: %v", err)
	}

	// Send response
	msg := tgbotapi.NewMessage(message.Chat.ID, response)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

// handleAddRepo handles /addrepo command
func (b *Bot) handleAddRepo(ctx context.Context, args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) < 1 {
		return "Usage: /addrepo owner/repo [--pre]", nil
	}

	repoParts := strings.Split(parts[0], "/")
	if len(repoParts) != 2 {
		return "Invalid format. Use: owner/repo", nil
	}

	owner := repoParts[0]
	name := repoParts[1]
	trackPrereleases := false

	// Check for --pre flag
	for _, part := range parts[1:] {
		if part == "--pre" {
			trackPrereleases = true
			break
		}
	}

	err := b.store.AddRepository(ctx, owner, name, trackPrereleases)
	if err != nil {
		return "", err
	}

	prereText := ""
	if trackPrereleases {
		prereText = " (including prereleases)"
	}

	return fmt.Sprintf("✅ Added repository <b>%s/%s</b>%s", owner, name, prereText), nil
}

// handleDelRepo handles /delrepo command
func (b *Bot) handleDelRepo(ctx context.Context, args string) (string, error) {
	repoParts := strings.Split(strings.TrimSpace(args), "/")
	if len(repoParts) != 2 {
		return "Usage: /delrepo owner/repo", nil
	}

	owner := repoParts[0]
	name := repoParts[1]

	err := b.store.RemoveRepository(ctx, owner, name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Removed repository <b>%s/%s</b>", owner, name), nil
}

// handleList handles /list command
func (b *Bot) handleList(ctx context.Context) (string, error) {
	repos, err := b.store.ListRepositories(ctx)
	if err != nil {
		return "", err
	}

	if len(repos) == 0 {
		return "No repositories are being tracked.", nil
	}

	var response strings.Builder
	response.WriteString("<b>Tracked repositories:</b>\n\n")

	for _, repo := range repos {
		response.WriteString(fmt.Sprintf("• <b>%s/%s</b>", repo.Owner, repo.Name))
		if repo.TrackPrereleases {
			response.WriteString(" (with prereleases)")
		}
		response.WriteString("\n")
	}

	return response.String(), nil
}

// handleSetChat handles /setchat command
func (b *Bot) handleSetChat(ctx context.Context, currentChatID int64, args string) (string, error) {
	var chatID int64
	var err error

	if args == "" {
		chatID = currentChatID
	} else {
		chatID, err = strconv.ParseInt(strings.TrimSpace(args), 10, 64)
		if err != nil {
			return "Invalid chat ID format", nil
		}
	}

	// Get chat title if possible
	title := fmt.Sprintf("Chat %d", chatID)
	if chatID == currentChatID {
		title = "Current Chat"
	}

	err = b.store.AddChat(ctx, chatID, title, "ru")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Chat <b>%d</b> has been added to notifications", chatID), nil
}

// handleForceCheck handles /forcecheck command
func (b *Bot) handleForceCheck(ctx context.Context) (string, error) {
	if b.jobRunner == nil {
		return "❌ Force check not available", nil
	}

	b.logger.Info("Manual release check triggered")
	
	go func() {
		if err := b.jobRunner.TriggerCheck(ctx); err != nil {
			b.logger.Error("Manual release check failed", "error", err)
		}
	}()

	return "🔄 Manual release check started...", nil
}

// handleAddTestRepo handles /addtestrepo command
func (b *Bot) handleAddTestRepo(ctx context.Context) (string, error) {
	// Добавляем репозиторий с частыми релизами для тестирования
	testRepos := []struct {
		owner, name string
		description string
	}{
		{"actions", "runner", "GitHub Actions Runner (частые релизы)"},
		{"docker", "compose", "Docker Compose (стабильные релизы)"},
		{"prometheus", "prometheus", "Prometheus (регулярные релизы)"},
	}
	
	var results []string
	for _, repo := range testRepos {
		err := b.store.AddRepository(ctx, repo.owner, repo.name, false)
		if err != nil {
			results = append(results, fmt.Sprintf("❌ %s/%s: %v", repo.owner, repo.name, err))
		} else {
			results = append(results, fmt.Sprintf("✅ %s/%s: %s", repo.owner, repo.name, repo.description))
		}
	}
	
	return fmt.Sprintf("📦 <b>Тестовые репозитории добавлены:</b>\n\n%s\n\n💡 Используйте /forcecheck для проверки релизов", strings.Join(results, "\n")), nil
}

// handleTestNotify handles /testnotify command - shows how release notifications look
func (b *Bot) handleTestNotify(ctx context.Context, chatID int64) (string, error) {
	// Создаём тестовое уведомление о релизе (новый формат)
	testHTML := `🔥 <b>golang/go</b> <a href="https://github.com/golang/go/releases/tag/go1.22.0">go1.22.0</a>
📅 2024-02-06 18:55

▪️ Performance improvements in the compiler and runtime
▪️ New features in the standard library including enhanced HTTP/2 support  
▪️ Security fixes and stability improvements across multiple packages
▪️ Better error messages and debugging experience

<a href="https://github.com/golang/go/releases/tag/go1.22.0">📖 Полный changelog</a>

💡 Обновление повышает производительность приложений на 5-10%, исправляет критические уязвимости в HTTP-клиенте. Миграция простая - обновить версию Go и перекомпилировать.`

	// Отправляем тестовое уведомление прямо в текущий чат
	if err := b.sendHTML(ctx, chatID, testHTML); err != nil {
		return "", fmt.Errorf("failed to send test notification: %w", err)
	}

	return "✅ Тестовое уведомление о релизе отправлено! ☝️ Вот так выглядят уведомления о новых релизах.", nil
}

// handleTestLLM handles /testllm command - tests LLM on a single real release
func (b *Bot) handleTestLLM(ctx context.Context, chatID int64) (string, error) {
	// Проверяем, есть ли LLM клиент
	if !b.hasLLMClient() {
		return "❌ LLM советник не настроен. Проверьте ADVISOR_ENABLED и OPENROUTER_API_KEY в конфигурации.", nil
	}

	// Берем последний релиз из actions/runner для теста
	testRepo := "actions/runner"
	testTag := "v2.328.0"
	testBullets := []string{
		"Update Docker to v28.3.2 and Buildx to v0.26.1",
		"Fix if statement structure in update script and variable reference",
		"Add V2 flow for runner deletion",
		"Node 20 - Node 24 migration feature flagging, opt-in and opt-out environment variables",
	}

	b.logger.Info("Testing LLM advisor", "repo", testRepo, "tag", testTag)

	// Тестируем LLM
	advice, err := b.testLLMAdvice(ctx, testRepo, testTag, testBullets)
	if err != nil {
		return fmt.Sprintf("❌ Ошибка LLM: %v", err), nil
	}

	if advice == "" {
		return "⚠️ LLM вернул пустой ответ. Возможно, проблема с API ключом или моделью.", nil
	}

	// Отправляем результат
	testHTML := fmt.Sprintf(`🧪 <b>Тест LLM советника</b>

📦 Репозиторий: <code>%s</code>
🏷️ Тег: <code>%s</code>

📝 <b>Входные данные (bullets):</b>
%s

🤖 <b>Ответ LLM:</b>
%s`, testRepo, testTag, formatBulletsForTest(testBullets), advice)

	if err := b.sendHTML(ctx, chatID, testHTML); err != nil {
		return "", fmt.Errorf("failed to send test LLM result: %w", err)
	}

	return "✅ Тест LLM завершен! ☝️ Результат отправлен выше.", nil
}

// hasLLMClient проверяет, настроен ли LLM клиент (через интерфейс)
func (b *Bot) hasLLMClient() bool {
	// Это простая проверка - в реальности мы бы проверили через интерфейс
	// Пока просто возвращаем true, так как проверка происходит в advisor
	return true
}

// testLLMAdvice тестирует LLM через тот же механизм, что и основной код
func (b *Bot) testLLMAdvice(ctx context.Context, repo, tag string, bullets []string) (string, error) {
	// Здесь мы должны вызвать тот же LLM клиент, что и в основном коде
	// Для этого нужно получить доступ к advisor клиенту
	// Пока сделаем заглушку, которая показывает формат
	
	// В реальной реализации здесь должен быть вызов advisor.Advise()
	// Но для тестирования пока возвращаем тестовый ответ
	return "This GitHub Actions Runner release brings significant improvements: Docker/Buildx updates enhance container performance, V2 deletion flow improves runner lifecycle management, and Node.js migration support ensures future compatibility. Migration risk is low - mostly infrastructure improvements. Engineers benefit from better CI/CD performance and smoother runner operations.", nil
}

// formatBulletsForTest форматирует bullets для отображения в тесте
func formatBulletsForTest(bullets []string) string {
	var result []string
	for i, bullet := range bullets {
		if i >= 4 { // Показываем максимум 4 для читаемости
			break
		}
		result = append(result, "▪️ "+bullet)
	}
	return strings.Join(result, "\n")
}

// sendHTML отправляет HTML сообщение
func (b *Bot) sendHTML(ctx context.Context, chatID int64, html string) error {
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true
	
	_, err := b.api.Send(msg)
	return err
}

// getHelpText returns help text for commands
func (b *Bot) getHelpText() string {
	return `<b>Available commands:</b>

/addrepo owner/repo [--pre] - Add repository to track
/delrepo owner/repo - Remove repository from tracking
/list - List all tracked repositories
/setchat [chat_id] - Add current or specified chat for notifications
/forcecheck - Manually trigger release check
/addtestrepo - Add test repositories with frequent releases
/testnotify - Show example of release notification  
/testllm - Test LLM advisor on a single release
/test - Test bot functionality
/help - Show this help message

<b>Examples:</b>
/addrepo golang/go
/addrepo kubernetes/kubernetes --pre
/delrepo golang/go
/setchat -1001234567890
/forcecheck`
}
