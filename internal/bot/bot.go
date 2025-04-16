package bot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"twitchannouncer/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"twitchannouncer/internal/config"
)

var userState = make(map[int64]string)
var data database.Data

func StartBot(cfg config.Config, bot *tgbotapi.BotAPI) {
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
				data.TelegramUsername = update.Message.From.UserName
			case "send":
				text := checkTwitchStream(data, cfg)
				bot.Send(tgbotapi.NewMessage(data.ChanelID, text))
			default:
				bot.Send(tgbotapi.NewMessage(chatID, "Неизвестная команда"))
			}
			continue
		}

		if userState[chatID] == "awaiting_username" {
			data.TwitchUsername = strings.TrimSpace(update.Message.Text)
			bot.Send(tgbotapi.NewMessage(chatID, "Отправьте ID канала"))
			userState[chatID] = "awaiting_chanel"
			continue
		}

		if userState[chatID] == "awaiting_chanel" {
			chanel_id := "-100" + update.Message.Text
			chanel_id_int, err := strconv.Atoi(chanel_id)
			if err != nil {
				panic(err)
				//TODO: handle error
			}
			data.ChanelID = int64(chanel_id_int)
			userState[chatID] = ""
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Оповещения о стримах %s успешно добавлены в канал %f", data.TwitchUsername, data.ChanelID)))
		}
	}
}

func checkTwitchStream(data database.Data, cfg config.Config) string {
	url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_login=%s", data.TwitchUsername)
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
		return fmt.Sprintf("Стример %s офлайн.", data.TwitchUsername)
	}

	stream := result.Data[0]
	return fmt.Sprintf("🎥 %s в эфире!\nИгра: %s\nНазвание: %s\nhttps://www.twitch.tv/%s\nhttps://www.twitch.tv/%s\nhttps://www.twitch.tv/%s\n", data.TwitchUsername, stream.GameName, stream.Title, data.TwitchUsername, data.TwitchUsername, data.TwitchUsername)
}
