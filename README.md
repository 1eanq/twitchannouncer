# 🎮 TwitchAnnouncer Bot

Телеграм-бот для автоматического уведомления о начале стримов на Twitch.

## 📦 Возможности

- Добавление Twitch-пользователей для отслеживания
- Автоматические уведомления в канал при старте стрима
- Удаление подписок
- Просмотр списка всех активных подписок

---

## 🚀 Быстрый старт

### 1. Клонируй репозиторий

```bash
git clone https://github.com/yourname/twitchannouncer.git
cd twitchannouncer
```

### 2. Настрой конфиг

Создай `.env` файл или отредактируй `config.Config` с данными:

- `TelegramBotToken` — токен Telegram-бота
- `TwitchClientID` — Twitch API Client ID
- `TwitchOAuthToken` — OAuth токен для Twitch API

### 3. Запусти бота

```bash
go run cmd/main.go
```

---

## ⚙️ Структура проекта

```
twitchannouncer/
├── cmd/
│   └── main/                # Точка входа
├── data/
│   └── database.db          # SQLite база данных
├── internal/
│   ├── bot/                 # Логика Telegram-бота
│   ├── config/              # Конфигурация
│   └── database/            # Работа с базой данных
```

---

## 💬 Команды бота

| Команда       | Описание                                            |
|---------------|-----------------------------------------------------|
| `/start`      | Приветствие                                         |
| `/help`       | Список доступных команд                             |
| `/new`        | ➕ Добавить подписку на Twitch пользователя          |
| `/list`       | 📋 Показать текущие активные подписки               |
| `/delete`     | ❌ Удалить подписку по Twitch-нику и ID канала      |

---

## ✅ Пример взаимодействия

1. `/new`
2. Ввод Twitch-username
3. Ввод ID канала (без `-100`, он добавляется автоматически)
4. Готово!

---

## 🔐 Зависимости

- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)

---

## 🧪 Тестирование

```bash
go test ./...
```

(тесты находятся в соответствующих папках, и требуют SQLite DB в `data/database.db`)

---

## 📬 Обратная связь

Нашёл баг или хочешь внести вклад? Открывай issue или pull request 🙌