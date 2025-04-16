package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"twitchannouncer/internal/bot"
	"twitchannouncer/internal/config"
)

func main() {
	cfg := config.LoadConfig("config.yaml")
	err := config.RefreshTwitchToken(&cfg, "config.yaml")
	if err != nil {
		log.Fatalf("Ошибка обновления Twitch токена: %v", err)
	}

	go config.RefreshTokenPeriodically(&cfg)

	bot_, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Panic(err)
	}

	bot_.Debug = true
	log.Printf("Authorized on account %s", bot_.Self.UserName)

	bot.StartBot(cfg, bot_)
}
