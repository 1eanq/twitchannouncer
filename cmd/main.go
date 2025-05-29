package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"net/http"
	"time"
	"twitchannouncer/internal/bot"
	"twitchannouncer/internal/config"
	"twitchannouncer/internal/database"
	"twitchannouncer/internal/yookassa"
)

func main() {
	cfg := config.LoadConfig("config.yaml")
	err := config.RefreshTwitchToken(&cfg, "config.yaml")
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

	botAPI, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Panic(err)
	}

	botAPI.Debug = true
	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	go bot.StartBot(cfg, botAPI, db)
	go bot.StartProExpiryChecker(db, 60*time.Minute)

	http.HandleFunc("/yookassa/webhook", yookassa.HandleWebhook(db, botAPI))

	log.Println("Starting HTTP server on :8081")
	err = http.ListenAndServe(":8081", nil)

	if err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
