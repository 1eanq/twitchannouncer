// Полная реализация DBInterface с логированием и структурой
package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const proDuration = 30 * 24 * time.Hour

type DBInterface interface {
	StoreData(userData UserData, subscriptionData SubscriptionData) error
	GetUserSubscriptions(id int64) ([]SubscriptionData, error)
	IfExists(data SubscriptionData) (bool, error)
	DeleteSubscriptionByID(id int) error
	GetAllSubscriptions() ([]SubscriptionData, error)
	GetAllTwitchUsernames() ([]string, error)
	GetAllChannelsForUser(username string) ([]int64, error)
	IsAdmin(id int) (bool, error)
	GetStreamData(username string) (*SubscriptionData, error)
	UpdateStreamStatus(username string, live, checked bool, latestMessageID int) error
	MakeUserPro(userID int64) error
	RemoveUserPro(userID int64) error
	IsUserPro(userID int64) (bool, time.Time, error)
	RemoveExpiredProUsers(bot *tgbotapi.BotAPI) error
	GetUserEmail(telegramID int64) (string, error)
	UpdateUserEmail(data UserData) error
}

type DB struct {
	Pool *pgxpool.Pool
}

func InitDatabase(connStr string) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Инициализация подключения к PostgreSQL")
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к PostgreSQL: %w", err)
	}

	tx := []string{
		`CREATE TABLE IF NOT EXISTS users (
			telegram_id BIGINT PRIMARY KEY,
			telegram_username TEXT,
			pro BOOLEAN DEFAULT FALSE,
			expires_at TIMESTAMP,
			admin BOOLEAN DEFAULT FALSE,
			email TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id SERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			channel_id BIGINT NOT NULL,
			channel_name TEXT,
			twitch_username TEXT NOT NULL,
			latest_message BIGINT NOT NULL DEFAULT 0,
			live BOOLEAN DEFAULT FALSE,
			checked BOOLEAN DEFAULT FALSE,
			UNIQUE(user_id, channel_id, twitch_username)
		)`,
	}

	for _, query := range tx {
		_, err = pool.Exec(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("ошибка при выполнении SQL: %w", err)
		}
	}

	log.Println("Подключение к PostgreSQL успешно, таблицы готовы")
	return &DB{Pool: pool}, nil
}

func (db *DB) StoreData(userData UserData, subscriptionData SubscriptionData) error {
	ctx := context.Background()
	log.Printf("Сохранение данных пользователя: %v, подписки: %v", userData, subscriptionData)

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (telegram_id, telegram_username)
			VALUES ($1, $2)
			ON CONFLICT (telegram_id) DO UPDATE SET telegram_username = EXCLUDED.telegram_username
	`, userData.TelegramID, userData.TelegramUsername)
	if err != nil {
		return fmt.Errorf("ошибка вставки/обновления пользователя: %w", err)
	}

	exists, err := db.IfExists(subscriptionData)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("такая подписка уже существует")
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, channel_id, channel_name, twitch_username)
			VALUES ($1, $2, $3, $4)
	`, subscriptionData.UserID, subscriptionData.ChannelID, subscriptionData.ChannelName, subscriptionData.TwitchUsername)
	if err != nil {
		return fmt.Errorf("ошибка вставки подписки: %w", err)
	}
	return nil
}

func (db *DB) GetUserSubscriptions(id int64) ([]SubscriptionData, error) {
	ctx := context.Background()
	log.Printf("Получение подписок пользователя %d", id)

	rows, err := db.Pool.Query(ctx, `
		SELECT id, twitch_username, channel_name, channel_id FROM subscriptions
			WHERE user_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка выборки: %w", err)
	}
	defer rows.Close()

	var subs []SubscriptionData
	for rows.Next() {
		var d SubscriptionData
		if err := rows.Scan(&d.ID, &d.TwitchUsername, &d.ChannelName, &d.ChannelID); err != nil {
			return nil, err
		}
		subs = append(subs, d)
	}
	return subs, nil
}

func (db *DB) IfExists(data SubscriptionData) (bool, error) {
	ctx := context.Background()
	var count int
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM subscriptions
			WHERE user_id = $1 AND twitch_username = $2 AND channel_id = $3
	`, data.UserID, data.TwitchUsername, data.ChannelID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке: %w", err)
	}
	return count > 0, nil
}

func (db *DB) DeleteSubscriptionByID(id int) error {
	ctx := context.Background()
	_, err := db.Pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1`, id)
	return err
}

func (db *DB) GetAllSubscriptions() ([]SubscriptionData, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT user_id, twitch_username, channel_id, channel_name, latest_message FROM subscriptions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SubscriptionData
	for rows.Next() {
		var d SubscriptionData
		if err := rows.Scan(&d.UserID, &d.TwitchUsername, &d.ChannelID, &d.ChannelName, &d.LatestMessageID); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, nil
}

func (db *DB) GetAllTwitchUsernames() ([]string, error) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT DISTINCT twitch_username FROM subscriptions`)
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
	rows, err := db.Pool.Query(ctx, `SELECT channel_id FROM subscriptions WHERE twitch_username = $1`, username)
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

func (db *DB) IsAdmin(id int) (bool, error) {
	ctx := context.Background()
	var admin bool
	err := db.Pool.QueryRow(ctx, `SELECT admin FROM users WHERE telegram_id = $1`, id).Scan(&admin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return admin, nil
}

func (db *DB) GetStreamData(username string) (*SubscriptionData, error) {
	ctx := context.Background()
	row := db.Pool.QueryRow(ctx, `
		SELECT user_id, twitch_username, live, checked, latest_message
			FROM subscriptions WHERE twitch_username = $1 LIMIT 1
	`, username)

	var data SubscriptionData
	err := row.Scan(&data.UserID, &data.TwitchUsername, &data.Live, &data.Checked, &data.LatestMessageID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить данные о стриме: %w", err)
	}
	return &data, nil
}

func (db *DB) UpdateStreamStatus(username string, live, checked bool, latestMessageID int) error {
	ctx := context.Background()
	_, err := db.Pool.Exec(ctx, `
		UPDATE subscriptions SET live = $1, checked = $2, latest_message = $3
			WHERE twitch_username = $4
	`, live, checked, latestMessageID, username)
	return err
}

func (db *DB) MakeUserPro(userID int64) error {
	expiry := time.Now().Add(proDuration)
	_, err := db.Pool.Exec(context.Background(), `
		INSERT INTO users (telegram_id, expires_at)
			VALUES ($1, $2)
			ON CONFLICT (telegram_id) DO UPDATE SET expires_at = EXCLUDED.expires_at;
	`, userID, expiry)
	return err
}

func (db *DB) RemoveUserPro(userID int64) error {
	_, err := db.Pool.Exec(context.Background(), `
		UPDATE users SET expires_at = NULL WHERE telegram_id = $1;
	`, userID)
	return err
}

func (db *DB) IsUserPro(userID int64) (bool, time.Time, error) {
	ctx := context.Background()
	var expiry time.Time
	err := db.Pool.QueryRow(ctx, `
		SELECT expires_at FROM users WHERE telegram_id = $1 AND expires_at > NOW()
	`, userID).Scan(&expiry)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, err
	}
	return true, expiry, nil
}

func (db *DB) RemoveExpiredProUsers(bot *tgbotapi.BotAPI) error {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, `SELECT telegram_id FROM users WHERE expires_at IS NOT NULL AND expires_at <= NOW();`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var expiredUserIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err == nil {
			expiredUserIDs = append(expiredUserIDs, userID)
		}
	}

	_, err = db.Pool.Exec(ctx, `UPDATE users SET expires_at = NULL WHERE expires_at IS NOT NULL AND expires_at <= NOW();`)
	if err != nil {
		return err
	}

	for _, userID := range expiredUserIDs {
		msg := tgbotapi.NewMessage(userID, "❌ Ваша подписка Pro истекла.")
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Не удалось отправить сообщение %d: %v", userID, err)
		}
	}
	return nil
}

func (db *DB) GetUserEmail(telegramID int64) (string, error) {
	var email *string
	err := db.Pool.QueryRow(context.Background(), `SELECT email FROM users WHERE telegram_id = $1`, telegramID).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("пользователь не найден")
		}
		return "", fmt.Errorf("ошибка запроса email: %w", err)
	}
	if email == nil || *email == "" {
		return "", fmt.Errorf("email не установлен")
	}
	return *email, nil
}

func (db *DB) UpdateUserEmail(data UserData) error {
	cmdTag, err := db.Pool.Exec(context.Background(), `UPDATE users SET email = $1 WHERE telegram_id = $2`, data.Email, data.TelegramID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении email: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("пользователь не найден")
	}
	return nil
}
