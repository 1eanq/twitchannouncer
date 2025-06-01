package yookassa

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"twitchannouncer/internal/database"
)

type WebhookNotification struct {
	Type   string `json:"type"`
	Event  string `json:"event"`
	Object struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Metadata struct {
			TelegramID string `json:"telegram_id"`
		} `json:"metadata"`
	} `json:"object"`
}

func HandleWebhook(db *database.DB, bot *tgbotapi.BotAPI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			log.Printf("Ошибка чтения тела запроса: %v", err)
			return
		}

		log.Printf("Получен webhook: %s", body)

		var notif WebhookNotification
		if err := json.Unmarshal(body, &notif); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			log.Printf("Ошибка декодирования JSON: %v", err)
			return
		}

		tgIDStr := notif.Object.Metadata.TelegramID
		if tgIDStr == "" {
			log.Println("Отсутствует telegram_id в metadata")
			w.WriteHeader(http.StatusOK)
			return
		}

		tgID, err := strconv.ParseInt(tgIDStr, 10, 64)
		if err != nil {
			log.Printf("Неверный telegram_id: %v", err)
			w.WriteHeader(http.StatusOK)
			return
		}

		switch notif.Event {
		case "payment.succeeded":
			err := db.MakeUserPro(tgID)
			if err != nil {
				log.Printf("Ошибка при установке Pro-подписки для %d: %v", tgID, err)
			} else {
				msg := tgbotapi.NewMessage(tgID, "✅ Ваша подписка Pro активирована! Спасибо за поддержку!")
				if _, err := bot.Send(msg); err != nil {
					log.Printf("Не удалось отправить сообщение пользователю %d: %v", tgID, err)
				}
				log.Printf("Pro активирована для пользователя %d", tgID)
			}
		default:
			log.Printf("Необработанное событие: %s", notif.Event)
		}

		w.WriteHeader(http.StatusOK)
	}
}
