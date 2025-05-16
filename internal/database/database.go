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

	// Создание таблицы
	_, err = pool.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		telegram_username TEXT NOT NULL,
		channel_id BIGINT NOT NULL,
		twitch_username TEXT NOT NULL,
		UNIQUE(twitch_username, channel_id)
	)`)

	if err != nil {
		return nil, fmt.Errorf("ошибка при создании таблицы: %w", err)
	}

	log.Println("Подключение к PostgreSQL установлено")
	return &DB{Pool: pool}, nil
}

func (db *DB) StoreData(data Data) error {
	ctx := context.Background()

	// Проверка существования
	var exists int
	err := db.Pool.QueryRow(ctx, `
		SELECT 1 FROM users WHERE twitch_username = $1 AND channel_id = $2
	`, data.TwitchUsername, data.ChannelID).Scan(&exists)

	if err == nil {
		return fmt.Errorf("такая подписка уже существует")
	}

	// Вставка
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (telegram_username, channel_id, twitch_username)
		VALUES ($1, $2, $3)
	`, data.TelegramUsername, data.ChannelID, data.TwitchUsername)

	if err != nil {
		return fmt.Errorf("ошибка вставки данных: %w", err)
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
