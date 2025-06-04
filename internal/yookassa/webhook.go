package yookassa

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

func HandleWebhook(db *database.DB, bot *tgbotapi.BotAPI, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		// Проверка подписи (пример)
		signature := r.Header.Get("Content-HMAC")
		if !verifySignature(body, signature, secret) {
			http.Error(w, "invalid signature", http.StatusForbidden)
			return
		}

		var notif WebhookNotification
		if err := json.Unmarshal(body, &notif); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		tgID, err := strconv.ParseInt(notif.Object.Metadata.TelegramID, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		switch notif.Event {
		case "payment.succeeded":
			isPro, _, err := db.IsUserPro(tgID)
			if err != nil {
				log.Println("Ошибка проверки Pro статуса:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !isPro {
				if err := db.MakeUserPro(tgID); err != nil {
					log.Println("Ошибка активации Pro:", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				msg := tgbotapi.NewMessage(tgID, "✅ Ваша подписка Pro активирована! Спасибо!")
				bot.Send(msg)
			}
		default:
			log.Printf("Необработанное событие: %s", notif.Event)
		}

		w.Write([]byte("ok"))
	}
}

func verifySignature(body []byte, signatureHeader string, secret string) bool {
	if signatureHeader == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)

	decodedSig, err := base64.StdEncoding.DecodeString(signatureHeader)
	if err != nil {
		return false
	}

	return hmac.Equal(decodedSig, expectedMAC)
}
