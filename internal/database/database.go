package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func InitDatabase(connStr string) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к PostgreSQL: %w", err)
	}

	// Создание таблицы пользователей
	_, err = pool.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS users (
		telegram_id BIGINT PRIMARY KEY,
		telegram_username TEXT,
		pro BOOLEAN DEFAULT FALSE,
		admin BOOLEAN DEFAULT FALSE
	)`)

	if err != nil {
		return nil, fmt.Errorf("ошибка при создании таблицы users: %w", err)
	}

	// Создание таблицы подписок
	_, err = pool.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS subscriptions (
		id SERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
		channel_id BIGINT NOT NULL,
		twitch_username TEXT NOT NULL,
		latest_message BIGINT NOT NULL DEFAULT 0,
		UNIQUE(user_id, channel_id, twitch_username)
	)`)

	if err != nil {
		return nil, fmt.Errorf("ошибка при создании таблицы subscriptions: %w", err)
	}

	log.Println("Подключение к PostgreSQL установлено и таблицы созданы")
	return &DB{Pool: pool}, nil
}

func (db *DB) StoreData(data Data) error {
	ctx := context.Background()

	// Вставка или обновление пользователя по Telegram ID
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (telegram_id, telegram_username)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE SET telegram_username = EXCLUDED.telegram_username
	`, data.TelegramID, data.TelegramUsername)

	if err != nil {
		return fmt.Errorf("ошибка вставки/обновления пользователя: %w", err)
	}

	// Проверка существования подписки
	var exists int
	err = db.Pool.QueryRow(ctx, `
		SELECT 1 FROM subscriptions WHERE user_id = $1 AND channel_id = $2 AND twitch_username = $3
	`, data.TelegramID, data.ChannelID, data.TwitchUsername).Scan(&exists)

	if err == nil {
		return fmt.Errorf("такая подписка уже существует")
	}

	// Вставка подписки
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, channel_id, twitch_username)
		VALUES ($1, $2, $3)
	`, data.TelegramID, data.ChannelID, data.TwitchUsername)

	if err != nil {
		return fmt.Errorf("ошибка вставки подписки: %w", err)
	}
	return nil
}

func (db *DB) GetUserSubscriptions(telegramUsername string) ([]Data, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `
		SELECT twitch_username, channel_id FROM users
		WHERE telegram_username = $1
	`, telegramUsername)
	if err != nil {
		return nil, fmt.Errorf("ошибка выборки: %w", err)
	}
	defer rows.Close()

	var subs []Data
	for rows.Next() {
		var d Data
		if err := rows.Scan(&d.TwitchUsername, &d.ChannelID); err != nil {
			return nil, err
		}
		subs = append(subs, d)
	}
	return subs, nil
}

func (db *DB) IfExists(data Data) (bool, error) {
	ctx := context.Background()
	var count int
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
		WHERE telegram_username = $1 AND twitch_username = $2 AND channel_id = $3
	`, data.TelegramUsername, data.TwitchUsername, data.ChannelID).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("ошибка при проверке: %w", err)
	}
	return count > 0, nil
}

func (db *DB) DeleteData(data Data) error {
	ctx := context.Background()
	exists, err := db.IfExists(data)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования")
	}
	if !exists {
		return fmt.Errorf("такой подписки не существует")
	}

	_, err = db.Pool.Exec(ctx, `
		DELETE FROM users
		WHERE telegram_username = $1 AND twitch_username = $2 AND channel_id = $3
	`, data.TelegramUsername, data.TwitchUsername, data.ChannelID)

	return err
}

func (db *DB) GetAllSubscriptions() ([]Data, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT twitch_username, channel_id FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Data
	for rows.Next() {
		var d Data
		if err := rows.Scan(&d.TwitchUsername, &d.ChannelID); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, nil
}

func (db *DB) GetAllTwitchUsernames() ([]string, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT DISTINCT twitch_username FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		usernames = append(usernames, username)
	}
	return usernames, nil
}

func (db *DB) GetAllChannelsForUser(username string) ([]int64, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT channel_id FROM users WHERE twitch_username = $1`, username)
	if err != nil {
		return nil, fmt.Errorf("ошибка выборки каналов: %w", err)
	}
	defer rows.Close()

	var channels []int64
	for rows.Next() {
		var ch int64
		if err := rows.Scan(&ch); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

func (db *DB) IsPro(username string) (bool, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT pro FROM users WHERE telegram_username = $1`, username)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if rows.Next() {
		var pro bool
		if err := rows.Scan(&pro); err != nil {
			return false, err
		}
		return pro, nil
	}

	// Пользователь не найден
	return false, fmt.Errorf("user not found")
}
