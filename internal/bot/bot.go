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
var deleteTemp = make(map[int64]database.Data)
var data database.Data

func StartBot(cfg config.Config, bot *tgbotapi.BotAPI, db *database.DB) {
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
				helpText := `📌 *Команды бота:*
								/help — Показать справку
								/new — ➕ Добавить Twitch-подписку
								/list — 📋 Посмотреть ваши подписки
								/delete — ❌ Удалить подписку по нику и ID`

				msg := tgbotapi.NewMessage(chatID, helpText)
				msg.ParseMode = "Markdown"
				bot.Send(msg)

			case "new":
				bot.Send(tgbotapi.NewMessage(chatID, "Напиши Twitch username:"))
				userState[chatID] = "awaiting_username"
				data.TelegramUsername = update.Message.From.UserName
			case "list":
				subs, err := db.GetUserSubscriptions(update.Message.From.UserName)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении списка ваших подписок."))
					continue
				}

				if len(subs) == 0 {
					bot.Send(tgbotapi.NewMessage(chatID, "У вас пока нет добавленных Twitch-юзернеймов."))
					continue
				}

				var msg strings.Builder
				msg.WriteString("Ваши активные подписки:\n")
				for _, sub := range subs {
					msg.WriteString(fmt.Sprintf("- %s → %d\n", sub.TwitchUsername, sub.ChannelID))
				}

				bot.Send(tgbotapi.NewMessage(chatID, msg.String()))

			case "delete":
				bot.Send(tgbotapi.NewMessage(chatID, "Введите Twitch username, который вы хотите удалить:"))
				userState[chatID] = "awaiting_delete_username"

			default:
				bot.Send(tgbotapi.NewMessage(chatID, "Неизвестная команда"))
			}
			continue
		}

		if userState[chatID] == "awaiting_username" {
			data.TwitchUsername = strings.ToLower(strings.TrimSpace(update.Message.Text))
			bot.Send(tgbotapi.NewMessage(chatID, "Отправьте ID канала"))
			userState[chatID] = "awaiting_channel"
			continue
		}

		if userState[chatID] == "awaiting_channel" {
			channelIDStr := "-100" + update.Message.Text
			channelIDInt, err := strconv.Atoi(channelIDStr)
			if err != nil {
				panic(err)
				//TODO: handle error
			}
			data.ChannelID = int64(channelIDInt)

			userState[chatID] = ""
			go monitorStreamLoop(data, cfg, bot)

			err = db.StoreData(data)
			if err != nil {
				if strings.Contains(err.Error(), "уже существует") {
					bot.Send(tgbotapi.NewMessage(chatID, "Такая запись уже существует!"))
					continue
				} else {
					bot.Send(tgbotapi.NewMessage(chatID, "Произошла ошибка при добавлении данных."))
					continue
				}
			}

			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Оповещения о стримах %s успешно добавлены в канал %d", data.TwitchUsername, data.ChannelID)))
		}

		if userState[chatID] == "awaiting_delete_username" {
			deleteTemp[chatID] = database.Data{
				TelegramUsername: update.Message.From.UserName,
				TwitchUsername:   strings.ToLower(strings.TrimSpace(update.Message.Text)),
			}
			bot.Send(tgbotapi.NewMessage(chatID, "Теперь введите ID канала, связанный с этим Twitch username:"))
			userState[chatID] = "awaiting_delete_channel"
			continue
		}

		if userState[chatID] == "awaiting_delete_channel" {
			dataToDelete := deleteTemp[chatID]
			channelIDStr := "-100" + update.Message.Text
			channelIDInt, err := strconv.Atoi(channelIDStr)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "Неверный формат ID канала."))
				continue
			}
			dataToDelete.TelegramUsername = deleteTemp[chatID].TelegramUsername
			dataToDelete.TwitchUsername = deleteTemp[chatID].TwitchUsername
			dataToDelete.ChannelID = int64(channelIDInt)

			err = db.DeleteData(dataToDelete)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, err.Error()))
				continue
			}

			bot.Send(tgbotapi.NewMessage(chatID, "Подписка успешно удалена!"))
			userState[chatID] = ""
			delete(deleteTemp, chatID)
			continue
		}

	}
}

func monitorStreamLoop(data database.Data, cfg config.Config, bot *tgbotapi.BotAPI) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var wasLive bool = false
	var latestMsgID int

	for {
		select {
		case <-ticker.C:
			live, streamInfo := checkStreamStatus(data.TwitchUsername, cfg)

			if live && !wasLive {
				text := fmt.Sprintf("🔴 %s начал стрим!\nИгра: %s\nНазвание: %s\nhttps://www.twitch.tv/%s", data.TwitchUsername, streamInfo.GameName, streamInfo.Title, data.TwitchUsername)
				msg, _ := bot.Send(tgbotapi.NewMessage(data.ChannelID, text))
				latestMsgID = msg.MessageID
			} else if !live && wasLive {
				_, err := bot.Request(tgbotapi.NewDeleteMessage(data.ChannelID, latestMsgID))
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
