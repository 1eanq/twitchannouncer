package database

import (
	"database/sql"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func InitDatabase(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("Error opening database: %v", err)
	}
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    telegram_username TEXT NOT NULL,
    channel_id INTEGER NOT NULL,
    twitch_username TEXT NOT NULL,
    UNIQUE(twitch_username, channel_id)
)
`)

	if err != nil {
		return nil, fmt.Errorf("ошибка при создании таблицы: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) StoreData(data Data) error {
	err := db.QueryRow(`
		SELECT channel_id FROM users WHERE twitch_username = ? AND channel_id = ?
	`, data.TwitchUsername, data.ChannelID).Scan(&data.ChannelID)

	if err == nil {
		return fmt.Errorf("Пользователь с таким twitch_username и channel_id уже существует")
	} else if err != sql.ErrNoRows {
		log.Printf("Ошибка при проверке данных: %v", err)
		return fmt.Errorf("не удалось проверить существование записи: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (telegram_username, channel_id, twitch_username)
		VALUES (?, ?, ?)
	`, data.TelegramUsername, data.ChannelID, data.TwitchUsername)

	if err != nil {
		log.Printf("Ошибка при добавлении данных в базу: %v", err)
		return fmt.Errorf("не удалось добавить данные в базу: %w", err)
	}

	return nil
}
