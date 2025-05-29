package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"twitchannouncer/internal/config"
	"twitchannouncer/internal/database"
	"twitchannouncer/internal/yookassa"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
		handleUpdate(bot, db, update)
	}
}

func handleUpdate(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID

	if update.Message.IsCommand() {
		handleCommand(bot, db, update)
		return
	}

	switch userState[chatID] {
	case "awaiting_username":
		handleAwaitingUsername(bot, update)
	case "awaiting_channel":
		handleAwaitingChannel(bot, db, update)
	case "awaiting_delete_username":
		handleAwaitingDeleteUsername(bot, update)
	case "awaiting_delete_channel":
		handleAwaitingDeleteChannel(bot, db, update)
	}
}

func handleCommand(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
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
		handleListCommand(bot, db, update)
	case "delete":
		bot.Send(tgbotapi.NewMessage(chatID, "Введите Twitch username, который вы хотите удалить:"))
		userState[chatID] = "awaiting_delete_username"
	case "pro":
		handleProCommand(bot, db, update)
	default:
		bot.Send(tgbotapi.NewMessage(chatID, "Неизвестная команда"))
	}
}

func handleListCommand(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	subs, err := db.GetUserSubscriptions(update.Message.From.ID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении списка ваших подписок."))
		fmt.Println(err.Error())
		return
	}

	if len(subs) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "У вас пока нет добавленных Twitch-юзернеймов."))
		return
	}

	var msg strings.Builder
	msg.WriteString("Ваши активные подписки:\n")
	for _, sub := range subs {
		msg.WriteString(fmt.Sprintf("- %s → %s\n", sub.TwitchUsername, sub.ChannelName))
	}

	bot.Send(tgbotapi.NewMessage(chatID, msg.String()))
}

func handleAwaitingUsername(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	data.TwitchUsername = strings.ToLower(strings.TrimSpace(update.Message.Text))
	bot.Send(tgbotapi.NewMessage(chatID, "Перешлите сообщение из канала\nКанал должен быть открытым!"))
	userState[chatID] = "awaiting_channel"
}

func handleAwaitingChannel(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	if update.Message.ForwardFromChat != nil && update.Message.ForwardFromChat.Type == "channel" {
		data.ChannelID = update.Message.ForwardFromChat.ID
		data.ChannelName = update.Message.ForwardFromChat.UserName
		userState[chatID] = ""

		err := db.StoreData(data)
		if err != nil {
			if strings.Contains(err.Error(), "уже существует") {
				bot.Send(tgbotapi.NewMessage(chatID, "Такая подписка уже существует!"))
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

func handleAwaitingDeleteUsername(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	deleteTemp[chatID] = database.UserData{
		TelegramID:       update.Message.From.ID,
		TelegramUsername: update.Message.From.UserName,
		TwitchUsername:   strings.ToLower(strings.TrimSpace(update.Message.Text)),
	}
	bot.Send(tgbotapi.NewMessage(chatID, "Теперь перешлите сообщение из канала, связанного с этим юзернеймом:"))
	userState[chatID] = "awaiting_delete_channel"
}

func handleAwaitingDeleteChannel(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	dataToDelete := deleteTemp[chatID]

	if update.Message.ForwardFromChat != nil && update.Message.ForwardFromChat.Type == "channel" {
		dataToDelete.ChannelID = update.Message.ForwardFromChat.ID
		dataToDelete.ChannelName = update.Message.ForwardFromChat.UserName

		err := db.DeleteData(dataToDelete)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, err.Error()))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "Подписка успешно удалена!"))
		}

		userState[chatID] = ""
		delete(deleteTemp, chatID)
	} else {
		bot.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, перешлите сообщение из канала, из которого нужно удалить подписку."))
	}
}

func handleProCommand(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	isPro, err := db.IsUserPro(userID)
	if err != nil {
		log.Printf("DB error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❗ Ошибка при проверке статуса. Попробуйте позже."))
		return
	}

	if isPro {
		bot.Send(tgbotapi.NewMessage(chatID, "✅ У вас уже активна подписка Pro. Спасибо!"))
		return
	}

	client := yookassa.NewClient()
	payURL, err := client.CreatePayment(userID)
	if err != nil {
		log.Printf("YooKassa error (user %d): %v", userID, err)
		bot.Send(tgbotapi.NewMessage(chatID, "❗ Ошибка при создании платежа. Попробуйте позже."))
		return
	}

	msg := fmt.Sprintf("💳 Для активации подписки Pro перейдите по ссылке и оплатите:\n%s", payURL)
	bot.Send(tgbotapi.NewMessage(chatID, msg))
}

func StartProExpiryChecker(bot *tgbotapi.BotAPI, db *database.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			err := db.RemoveExpiredProUsers(bot)
			if err != nil {
				log.Printf("❗ Ошибка при удалении просроченных подписок: %v", err)
			}
		}
	}()
}
