package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"log"
	"time"
	"twitchannouncer/internal/bot"
	"twitchannouncer/internal/config"
	"twitchannouncer/internal/database"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	cfg := config.LoadConfig("config.yaml")
	err = config.RefreshTwitchToken(&cfg, "config.yaml")
	if err != nil {
		log.Fatalf("Ошибка обновления Twitch токена: %v", err)
	}

	go config.RefreshTokenPeriodically(&cfg)

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
	)

	db, err := database.InitDatabase(connStr)
	if err != nil {
		log.Fatal(err)
	}

	bot_, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Panic(err)
	}

	bot_.Debug = true
	log.Printf("Authorized on account %s", bot_.Self.UserName)

	bot.StartBot(cfg, bot_, db)

	bot.StartProExpiryChecker(db, 60*time.Minute)
}
