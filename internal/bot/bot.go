package bot

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"twitchannouncer/internal/config"
	"twitchannouncer/internal/database"
	"twitchannouncer/internal/yookassa"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var userState = make(map[int64]string)
var userData database.UserData
var subscriptionData database.SubscriptionData

func StartBot(cfg config.Config, bot *tgbotapi.BotAPI, db *database.DB) {
	ctx := context.Background()
	monitor := NewMonitor(bot, db, cfg)
	go monitor.Start(ctx, 10*time.Second)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			handleCallbackQuery(bot, db, update.CallbackQuery)
			continue
		}
		if update.Message == nil {
			continue
		}
		handleUpdate(bot, db, update)
	}
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, db *database.DB, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	userID := callback.From.ID
	data := callback.Data

	switch {
	case strings.HasPrefix(data, "list_page_"):
		pageStr := strings.TrimPrefix(data, "list_page_")
		page, _ := strconv.Atoi(pageStr)
		subs, _ := db.GetUserSubscriptions(userID)

		msgText, keyboard := buildSubscriptionPage(subs, page)
		edit := tgbotapi.NewEditMessageText(chatID, messageID, msgText)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		bot.Send(edit)

	case strings.HasPrefix(data, "delete_sub_"):
		idStr := strings.TrimPrefix(data, "delete_sub_")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Printf("Неверный ID подписки: %v", err)
			return
		}

		subscriptions, _ := db.GetUserSubscriptions(userID)
		var sub *database.SubscriptionData
		for _, s := range subscriptions {
			if s.ID == id {
				sub = &s
				break
			}
		}
		if sub == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❗ Подписка не найдена."))
			return
		}

		text := fmt.Sprintf("❗ Вы уверены, что хотите удалить подписку `%s → %s`?", sub.TwitchUsername, sub.ChannelName)
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text,
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", fmt.Sprintf("confirm_sub_%d", sub.ID)),
					tgbotapi.NewInlineKeyboardButtonData("🔙 Отмена", "list_page_0"),
				),
			),
		)
		edit.ParseMode = "Markdown"
		bot.Send(edit)

	case strings.HasPrefix(data, "confirm_sub_"):
		idStr := strings.TrimPrefix(data, "confirm_sub_")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Printf("Неверный ID подписки: %v", err)
			bot.Send(tgbotapi.NewCallback(callback.ID, "Ошибка при удалении подписки"))
			return
		}

		err = db.DeleteSubscriptionByID(id)
		if err != nil {
			log.Printf("Ошибка удаления подписки: %v", err)
			bot.Send(tgbotapi.NewCallback(callback.ID, "Ошибка при удалении подписки"))
			return
		}

		text := "✅ Подписка успешно удалена."
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		bot.Send(edit)
	}

	bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func buildSubscriptionPage(subs []database.SubscriptionData, page int) (string, tgbotapi.InlineKeyboardMarkup) {
	const perPage = 5
	start := page * perPage
	end := start + perPage
	if end > len(subs) {
		end = len(subs)
	}
	paginated := subs[start:end]

	var msg strings.Builder
	msg.WriteString("Ваши активные подписки:\n")
	rows := [][]tgbotapi.InlineKeyboardButton{}

	for _, sub := range paginated {
		text := fmt.Sprintf("%s → %s", sub.TwitchUsername, sub.ChannelName)
		callbackData := fmt.Sprintf("delete_sub_%d", sub.ID) // только ID

		button := tgbotapi.NewInlineKeyboardButtonData(text, callbackData)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}

	navRow := []tgbotapi.InlineKeyboardButton{}
	if page > 0 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", fmt.Sprintf("list_page_%d", page-1)))
	}
	if end < len(subs) {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("Вперёд ➡", fmt.Sprintf("list_page_%d", page+1)))
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return msg.String(), keyboard
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
	case "awaiting_email":
		handleAwaitingEmail(bot, db, update)
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
			/list — 📋 Посмотреть ваши подписки`
		msg := tgbotapi.NewMessage(chatID, helpText)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
	case "new":
		bot.Send(tgbotapi.NewMessage(chatID, "Напиши Twitch username:"))
		userState[chatID] = "awaiting_username"
		userData.TelegramID = update.Message.From.ID
		userData.TelegramUsername = update.Message.From.UserName
	case "list":
		subs, err := db.GetUserSubscriptions(update.Message.From.ID)
		if err != nil || len(subs) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "У вас пока нет добавленных Twitch-юзернеймов."))
			return
		}
		msgText, keyboard := buildSubscriptionPage(subs, 0)
		msg := tgbotapi.NewMessage(chatID, msgText)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	case "delete":
		bot.Send(tgbotapi.NewMessage(chatID, "Введите Twitch username, который вы хотите удалить:"))
		userState[chatID] = "awaiting_delete_username"
	case "pro":
		handleProCommand(bot, db, update)
	default:
		bot.Send(tgbotapi.NewMessage(chatID, "Неизвестная команда"))
	}
}

func handleAwaitingUsername(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	subscriptionData.TwitchUsername = strings.ToLower(strings.TrimSpace(update.Message.Text))
	bot.Send(tgbotapi.NewMessage(chatID, "Перешлите сообщение из канала\nКанал должен быть открытым!"))
	userState[chatID] = "awaiting_channel"
}

func handleAwaitingChannel(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	if update.Message.ForwardFromChat != nil && update.Message.ForwardFromChat.Type == "channel" {
		subscriptionData.ChannelID = update.Message.ForwardFromChat.ID
		subscriptionData.ChannelName = update.Message.ForwardFromChat.UserName
		userState[chatID] = ""

		subscriptionData.UserID = userData.TelegramID
		err := db.StoreData(userData, subscriptionData)
		if err != nil {
			text := "Произошла ошибка при добавлении данных."
			log.Println(err)
			bot.Send(tgbotapi.NewMessage(chatID, text))
			return
		}

		text := fmt.Sprintf("Оповещения о стримах %s успешно добавлены в канал @%s", subscriptionData.TwitchUsername, subscriptionData.ChannelName)
		bot.Send(tgbotapi.NewMessage(chatID, text))
	} else {
		bot.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, перешлите сообщение из канала, чтобы я мог получить его ID."))
	}
}

func handleAwaitingEmail(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	email := strings.TrimSpace(update.Message.Text)

	if !isValidEmail(email) {
		bot.Send(tgbotapi.NewMessage(chatID, "❗ Пожалуйста, введите корректный email."))
		return
	}
	userData.TelegramID = update.Message.From.ID
	userData.Email = email

	db.UpdateUserEmail(userData)

	userState[chatID] = ""

}

func handleProCommand(bot *tgbotapi.BotAPI, db *database.DB, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	description := `🌟 *Подписка Pro* даёт вам:
- 🔔 Уведомления без ограничений
- 📈 Приоритетную обработку запросов
- 🚫 Отключение всей рекламы
Стоимость — всего *50₽ в месяц*`

	isPro, expiry, err := db.IsUserPro(userID)
	if err != nil {
		log.Printf("DB error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❗ Ошибка при проверке статуса. Попробуйте позже."))
		return
	}

	if isPro {
		text := fmt.Sprintf("%s\n\n✅ У вас уже активна подписка *Pro* до *%s*.", description, expiry.Format("02.01.2006"))
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
		return
	}

	email, err := db.GetUserEmail(userID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите email")
		bot.Send(msg)
		userState[chatID] = "awaiting_email"
		return
	}

	if email == "" {
		msg := tgbotapi.NewMessage(chatID, "❗ Email не может быть пустым. Пожалуйста, добавьте email в профиле и попробуйте снова.")
		bot.Send(msg)
		return
	}

	client := yookassa.NewClient()
	payURL, err := client.CreatePayment(userID, email)
	if err != nil {
		log.Printf("YooKassa error (user %d): %v", userID, err)
		bot.Send(tgbotapi.NewMessage(chatID, "❗ Ошибка при создании платежа. Попробуйте позже."))
		return
	}

	amount := "50₽"
	msgText := fmt.Sprintf("%s\n\n💳 Нажмите кнопку ниже, чтобы оплатить *%s* и активировать подписку:", description, amount)

	button := tgbotapi.NewInlineKeyboardButtonURL("Оплатить "+amount, payURL)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(button),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
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

func isValidEmail(email string) bool {
	// Простейшая проверка email регулярным выражением
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}
