package database

import (
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const proDuration = 30 * 24 * time.Hour

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

func (db *DB) StoreData(userData UserData, subscriptionData SubscriptionData) error {
	ctx := context.Background()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (telegram_id, telegram_username)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE SET telegram_username = EXCLUDED.telegram_username
	`, userData.TelegramID, userData.TelegramUsername)

	if err != nil {
		return fmt.Errorf("ошибка вставки/обновления пользователя: %w", err)
	}

	var exists int
	err = db.Pool.QueryRow(ctx, `
	SELECT 1 FROM subscriptions WHERE user_id = $1 AND channel_id = $2 AND twitch_username = $3
`, subscriptionData.UserID, subscriptionData.ChannelID, subscriptionData.TwitchUsername).Scan(&exists)

	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("ошибка при проверке существующей подписки: %w", err)
	}

	if err == nil {
		return fmt.Errorf("такая подписка уже существует")
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, channel_id, channel_name, twitch_username)
		VALUES ($1, $2, $3, $4)
	`, subscriptionData.UserID, subscriptionData.ChannelID, subscriptionData.ChannelName, subscriptionData.TwitchUsername)

	if err != nil {
		if err != nil {
			log.Printf("Ошибка при вставке подписки: %v", err)
			return fmt.Errorf("ошибка вставки подписки: %w", err)
		}

	}
	return nil
}

func (db *DB) GetUserSubscriptions(id int64) ([]SubscriptionData, error) {
	ctx := context.Background()
	log.Printf("Получение списка подписок для %d", id)
	rows, err := db.Pool.Query(ctx, `
		SELECT id, twitch_username, channel_name, channel_id FROM subscriptions
		WHERE user_id = $1
	`, id)
	if err != nil {
		log.Printf("Ошибка получения списка подписок: %v", err)
		return nil, fmt.Errorf("ошибка выборки: %w", err)
	}
	defer rows.Close()

	var subs []SubscriptionData
	for rows.Next() {
		var d SubscriptionData
		if err := rows.Scan(&d.ID, &d.TwitchUsername, &d.ChannelName, &d.ChannelID); err != nil {
			log.Printf("Ошибка сканирования: %v", err)
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
		log.Printf("ошибка при проверке: %v", err)
		return false, fmt.Errorf("ошибка при проверке: %w", err)
	}
	return count > 0, nil
}

func (db *DB) DeleteSubscriptionByID(id int) error {
	ctx := context.Background()

	_, err := db.Pool.Exec(ctx, `
		DELETE FROM subscriptions
		WHERE id = $1
	`, id)

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
	rows, err := db.Pool.Query(ctx, `SELECT admin FROM users WHERE telegram_id = $1`, id)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if rows.Next() {
		var admin bool
		if err := rows.Scan(&admin); err != nil {
			return false, err
		}
		return admin, nil
	}

	return false, fmt.Errorf("user not found")
}

func (db *DB) GetStreamData(username string) (*SubscriptionData, error) {
	ctx := context.Background()
	row := db.Pool.QueryRow(ctx, `
		SELECT user_id, twitch_username, live, checked, latest_message
		FROM subscriptions
		WHERE twitch_username = $1
		LIMIT 1
	`, username)

	var data SubscriptionData
	err := row.Scan(&data.UserID, &data.TwitchUsername, &data.Live, &data.Checked, &data.LatestMessageID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить данные о стриме: %w", err)
	}

	return &data, nil
}

func (db *DB) UpdateStreamStatus(username string, live bool, checked bool, latestMessageID int) error {
	ctx := context.Background()
	_, err := db.Pool.Exec(ctx, `
		UPDATE subscriptions
		SET live = $1, checked = $2, latest_message = $3
		WHERE twitch_username = $4
	`, live, checked, latestMessageID, username)
	return err
}

func (db *DB) MakeUserPro(userID int64) error {
	expiry := time.Now().Add(proDuration)

	_, err := db.Pool.Exec(context.Background(), `
		INSERT INTO users (telegram_id, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE
		SET expires_at = EXCLUDED.expires_at;
	`, userID, expiry)

	return err
}

func (db *DB) RemoveUserPro(userID int64) error {
	_, err := db.Pool.Exec(context.Background(), `
		UPDATE users
		SET expires_at = NULL
		WHERE telegram_id = $1;
	`, userID)

	return err
}

func (db *DB) IsUserPro(userID int64) (bool, error) {
	var expiresAt *time.Time

	err := db.Pool.QueryRow(context.Background(), `
		SELECT expires_at FROM users WHERE telegram_id = $1;
	`, userID).Scan(&expiresAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return expiresAt != nil && expiresAt.After(time.Now()), nil
}

func (db *DB) RemoveExpiredProUsers(bot *tgbotapi.BotAPI) error {
	rows, err := db.Pool.Query(context.Background(), `
		SELECT telegram_id FROM users
		WHERE expires_at IS NOT NULL AND expires_at <= NOW();
	`)
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

	_, err = db.Pool.Exec(context.Background(), `
		UPDATE users
		SET expires_at = NULL
		WHERE expires_at IS NOT NULL AND expires_at <= NOW();
	`)
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
