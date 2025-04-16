package bot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"twitchannouncer/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"twitchannouncer/internal/config"
)

type StreamInfo struct {
	Title       string
	ViewerCount int
	GameName    string
}

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
			go monitorStreamLoop(data, cfg, bot)
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Оповещения о стримах %s успешно добавлены в канал %d", data.TwitchUsername, data.ChanelID)))
		}
	}
}

func monitorStreamLoop(data database.Data, cfg config.Config, bot *tgbotapi.BotAPI) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	var wasLive bool = false
	var latestMsgID int

	for {
		select {
		case <-ticker.C:
			live, streamInfo := checkStreamStatus(data.TwitchUsername, cfg)

			// ТОЛЬКО если статус изменился
			if live && !wasLive {
				text := fmt.Sprintf("🔴 %s начал стрим!\nИгра: %s\nНазвание: %s\nhttps://www.twitch.tv/%s", data.TwitchUsername, streamInfo.GameName, streamInfo.Title, data.TwitchUsername)
				msg, _ := bot.Send(tgbotapi.NewMessage(data.ChanelID, text))
				latestMsgID = msg.MessageID
			} else if !live && wasLive {
				_, err := bot.Request(tgbotapi.NewDeleteMessage(data.ChanelID, latestMsgID))
				if err != nil {
					fmt.Println("Ошибка удаления!")
				}
			}

			wasLive = live
		}
	}
}

func checkStreamStatus(username string, cfg config.Config) (bool, StreamInfo) {
	url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_login=%s", username)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Client-ID", cfg.TwitchClientID)
	req.Header.Set("Authorization", "Bearer "+cfg.TwitchOAuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, StreamInfo{}
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
		return false, StreamInfo{}
	}

	if len(result.Data) == 0 {
		return false, StreamInfo{}
	}

	stream := result.Data[0]
	return true, StreamInfo{
		Title:       stream.Title,
		ViewerCount: stream.ViewerCount,
		GameName:    stream.GameName,
	}
}
