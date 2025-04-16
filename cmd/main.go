package main

import (
	"log"
	"time"

	"twitchannouncer/internal/bot"
	"twitchannouncer/internal/config"
)

func main() {
	// Загружаем конфиг
	cfg := config.LoadConfig("config.yaml")

	// Обновляем токен сразу при старте, если нужно
	err := config.RefreshTwitchToken(&cfg, "config.yaml")
	if err != nil {
		log.Fatalf("Ошибка обновления Twitch токена: %v", err)
	}

	// Запуск горутины для проверки токена каждые 5 минут
	go refreshTokenPeriodically(&cfg)

	// Запуск бота
	telegram.StartBot(cfg)
}

// Функция, которая будет проверять и обновлять токен каждые 60 минут
func refreshTokenPeriodically(cfg *config.Config) {
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Проверка и обновление токена
			err := config.RefreshTwitchToken(cfg, "config.yaml")
			if err != nil {
				log.Printf("Не удалось обновить Twitch токен: %v", err)
			}
		}
	}
}
