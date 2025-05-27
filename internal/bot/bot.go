package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"twitchannouncer/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"twitchannouncer/internal/config"
)

var userState = make(map[int64]string)
var deleteTemp = make(map[int64]database.UserData)
var data database.UserData

func StartBot(cfg config.Config, bot *tgbotapi.BotAPI, db *database.DB) {
	ctx := context.Background()
	monitor := NewMonitor(bot, db, cfg)
	go monitor.Start(ctx, 5*time.Second)

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
				data.TelegramID = update.Message.From.ID
				data.TelegramUsername = update.Message.From.UserName

			case "list":
				subs, err := db.GetUserSubscriptions(update.Message.From.ID)
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
			bot.Send(tgbotapi.NewMessage(chatID, "Перешлите сообщение из канала"))
			userState[chatID] = "awaiting_channel"
			continue
		}

		if userState[chatID] == "awaiting_channel" {
			if update.Message.ForwardFromChat != nil && update.Message.ForwardFromChat.Type == "channel" {
				channelID := update.Message.ForwardFromChat.ID
				channelName := update.Message.ForwardFromChat.UserName
				fmt.Println(channelID, channelName)
				data.ChannelID = channelID
				data.ChannelName = channelName

				userState[chatID] = ""

				err := db.StoreData(data)
				if err != nil {
					if strings.Contains(err.Error(), "уже существует") {
						bot.Send(tgbotapi.NewMessage(chatID, "Такая запись уже существует!"))
					} else {
						bot.Send(tgbotapi.NewMessage(chatID, "Произошла ошибка при добавлении данных."))
					}
					return
				}

				bot.Send(tgbotapi.NewMessage(chatID,
					fmt.Sprintf("Оповещения о стримах %s успешно добавлены в канал @%s", data.TwitchUsername, data.ChannelName)))

			} else {
				bot.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, перешлите сообщение из канала, чтобы я мог получить его ID."))
			}
		}

		if userState[chatID] == "awaiting_delete_username" {
			deleteTemp[chatID] = database.UserData{
				TelegramID:       update.Message.From.ID,
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
