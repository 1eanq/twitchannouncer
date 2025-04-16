package telegram

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"twitchannouncer/internal/config"
)

var userState = make(map[int64]string)

func StartBot(cfg config.Config) {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				bot.Send(tgbotapi.NewMessage(chatID, "Вас приветствует бот для автоматической отправки уведомлений о стримах.\n/help для просмотра доступных комманд!"))
			case "help":
				bot.Send(tgbotapi.NewMessage(chatID, "/new — проверить Twitch стрим по нику"))
			case "new":
				bot.Send(tgbotapi.NewMessage(chatID, "Напиши Twitch username:"))
				userState[chatID] = "awaiting_username"
			default:
				bot.Send(tgbotapi.NewMessage(chatID, "Неизвестная команда"))
			}
			continue
		}

		if userState[chatID] == "awaiting_username" {
			username := strings.TrimSpace(update.Message.Text)
			userState[chatID] = ""
			text := checkTwitchStream(username, cfg)
			bot.Send(tgbotapi.NewMessage(chatID, text))
			continue
		}

		bot.Send(tgbotapi.NewMessage(chatID, "Напиши /new чтобы проверить Twitch стрим"))
	}
}

func checkTwitchStream(username string, cfg config.Config) string {
	url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_login=%s", username)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Client-ID", cfg.TwitchClientID)
	req.Header.Set("Authorization", "Bearer "+cfg.TwitchOAuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "Ошибка подключения к Twitch"
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Type        string `json:"type"`
			Title       string `json:"title"`
			ViewerCount int    `json:"viewer_count"`
			GameName    string `json:"game_name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "Ошибка обработки данных Twitch"
	}

	if len(result.Data) == 0 {
		return fmt.Sprintf("Стример %s офлайн.", username)
	}

	stream := result.Data[0]
	return fmt.Sprintf("🎥 %s в эфире!\nИгра: %s\nНазвание: %s\nhttps://www.twitch.tv/%s\nhttps://www.twitch.tv/%s\nhttps://www.twitch.tv/%s\n", username, stream.GameName, stream.Title, username, username, username)
}
