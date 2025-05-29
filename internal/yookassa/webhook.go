package yookassa

import (
	"encoding/json"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"io"
	"log"
	"net/http"
	"strconv"

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
	log.Println("➡️ Получен запрос от YooKassa на вебхук")
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		log.Printf("Webhook body: %s", string(body))

		var notif WebhookNotification
		err = json.Unmarshal(body, &notif)
		if err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if notif.Event == "payment.succeeded" {
			tgID, err := strconv.ParseInt(notif.Object.Metadata.TelegramID, 10, 64)
			if err != nil {
				log.Printf("Неверный telegram_id: %v", err)
				return
			}

			err = db.MakeUserPro(tgID)
			if err != nil {
				log.Printf("Ошибка активации Pro: %v", err)
				return
			}

			msg := tgbotapi.NewMessage(tgID, "✅ Ваша подписка Pro активирована! Спасибо.")
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}

			log.Printf("Пользователь %d успешно активировал Pro", tgID)
		}

		w.WriteHeader(http.StatusOK)
	}
}
