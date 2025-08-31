# Telegram Release Bot

Telegram-бот на Go для отслеживания новых релизов GitHub репозиториев и отправки уведомлений в Telegram чаты.

## Возможности

- 🔍 Отслеживание релизов GitHub репозиториев каждые N минут
- 📱 Отправка уведомлений в Telegram чаты в HTML формате
- 🚀 Поддержка ETag для экономии API квоты GitHub
- 🤖 Опциональный LLM-советник через OpenRouter для анализа релизов
- ⚙️ Команды администрирования через Telegram бота
- 📊 Структурированное логирование
- 🗄️ SQLite база данных (без CGO)
- 🐳 Docker поддержка

## Быстрый старт

### 1. Клонирование и настройка

```bash
git clone <repository>
cd tg-release-bot
cp env.example .env
```

### 2. Настройка переменных окружения

Отредактируйте `.env` файл:

```bash
# Обязательные настройки
GITHUB_TOKEN=ghp_your_github_token_here
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
DEFAULT_CHAT_ID=-1001234567890

# Опциональные настройки
ADVISOR_ENABLED=1
OPENROUTER_API_KEY=sk-or-your_openrouter_api_key_here
ALLOWED_USER_IDS=123456789,987654321
```

### 3. Запуск с Docker

```bash
# Создать директорию для данных
mkdir -p data

# Запустить бота
docker-compose up -d

# Посмотреть логи
docker-compose logs -f
```

### 4. Запуск без Docker

```bash
# Установить зависимости
go mod download

# Запустить
go run cmd/bot/main.go
```

## Настройка репозиториев

### Через команды бота (если настроены ALLOWED_USER_IDS)

```
/addrepo golang/go
/addrepo kubernetes/kubernetes --pre
/list
/delrepo golang/go
/setchat -1001234567890
```

### Через базу данных

```sql
INSERT INTO repos (owner, name, track_prereleases) VALUES ('golang', 'go', 0);
INSERT INTO chats (id, title, language) VALUES (-1001234567890, 'My Chat', 'ru');
```

## Команды бота

| Команда | Описание | Пример |
|---------|----------|--------|
| `/addrepo owner/repo [--pre]` | Добавить репозиторий | `/addrepo golang/go --pre` |
| `/delrepo owner/repo` | Удалить репозиторий | `/delrepo golang/go` |
| `/list` | Список отслеживаемых репозиториев | `/list` |
| `/setchat [chat_id]` | Добавить чат для уведомлений | `/setchat -1001234567890` |
| `/test` | Тест работы бота | `/test` |
| `/help` | Помощь | `/help` |

## Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `GITHUB_TOKEN` | GitHub Personal Access Token | **Обязательно** |
| `TELEGRAM_BOT_TOKEN` | Telegram Bot Token | **Обязательно** |
| `DEFAULT_CHAT_ID` | ID чата по умолчанию | `0` |
| `POLL_INTERVAL_MINUTES` | Интервал проверки в минутах | `10` |
| `TIMEZONE` | Часовой пояс | `Europe/Amsterdam` |
| `ADVISOR_ENABLED` | Включить LLM советник | `0` |
| `OPENROUTER_API_KEY` | API ключ OpenRouter | `` |
| `OPENROUTER_MODEL` | Модель LLM | `openrouter/anthropic/claude-3-haiku` |
| `ALLOWED_USER_IDS` | ID пользователей для команд | `` |
| `MAX_CHANGELOG_CHARS` | Макс. символов в changelog | `2500` |
| `MAX_BULLETS` | Макс. пунктов из changelog | `8` |
| `DB_PATH` | Путь к базе данных | `./releases.db` |

### GitHub Token

Создайте Personal Access Token на GitHub:
1. Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate new token
3. Выберите scope: `public_repo` (для публичных репозиториев)

### Telegram Bot

1. Создайте бота через [@BotFather](https://t.me/botfather)
2. Получите token
3. Добавьте бота в нужный чат
4. Получите chat_id (можно через [@userinfobot](https://t.me/userinfobot))

## Архитектура

```
cmd/bot/                 # Основное приложение
internal/
  ├── config/           # Конфигурация
  ├── logging/          # Логирование
  ├── db/               # База данных SQLite
  ├── github/           # GitHub API клиент
  ├── telegram/         # Telegram интеграция
  ├── compose/          # Формирование сообщений
  ├── scheduler/        # Планировщик задач
  └── advisor/          # LLM советник
```

## Формат сообщений

```
🚀 Новый релиз: golang/go v1.22.0
📅 Дата: 2024-02-06 18:55

📝 Заметки:
• Performance improvements in the compiler
• New features in the standard library
• Bug fixes and stability improvements

🔗 Читать changelog

💡 Польза для инженеров:
Релиз Go 1.22 приносит улучшения производительности...
```

## Развертывание

### Docker Compose (рекомендуется)

```bash
# Клонировать и настроить
git clone <repository>
cd tg-release-bot
cp env.example .env
# Отредактировать .env

# Запустить
docker-compose up -d
```

### Systemd Service

```ini
[Unit]
Description=Telegram Release Bot
After=network.target

[Service]
Type=simple
User=bot
WorkingDirectory=/opt/tg-release-bot
ExecStart=/opt/tg-release-bot/tg-release-bot
Restart=always
RestartSec=10
EnvironmentFile=/opt/tg-release-bot/.env

[Install]
WantedBy=multi-user.target
```

## Мониторинг

Бот логирует все операции в структурированном формате:

```bash
# Посмотреть логи Docker
docker-compose logs -f

# Фильтровать ошибки
docker-compose logs | grep ERROR

# Посмотреть статистику
docker-compose logs | grep "Release check job completed"
```

## Troubleshooting

### Частые проблемы

1. **GitHub API Rate Limit**
   - Убедитесь, что используется валидный GITHUB_TOKEN
   - Проверьте лимиты: `curl -H "Authorization: Bearer $TOKEN" https://api.github.com/rate_limit`

2. **Telegram API ошибки**
   - Проверьте валидность TELEGRAM_BOT_TOKEN
   - Убедитесь, что бот добавлен в чат

3. **База данных**
   - Убедитесь в правах доступа к DB_PATH
   - Проверьте свободное место на диске

### Логи

```bash
# Включить debug логи
ENV=development docker-compose up

# Посмотреть конкретную ошибку
docker-compose logs | grep "error"
```

## Разработка

### Запуск для разработки

```bash
# Установить зависимости
go mod download

# Запустить с live reload (air)
go install github.com/cosmtrek/air@latest
air

# Или обычный запуск
go run cmd/bot/main.go
```

### Тестирование

```bash
# Запустить тесты
go test ./...

# С покрытием
go test -cover ./...
```

## Лицензия

MIT License
