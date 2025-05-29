package yookassa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type YooKassaPaymentRequest struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Confirmation struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	} `json:"confirmation"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

type YooKassaPaymentResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Confirmation struct {
		Type string `json:"type"`
		URL  string `json:"confirmation_url"`
	} `json:"confirmation"`
}

func CreateYooKassaPayment(telegramID int64) (string, error) {
	reqBody := YooKassaPaymentRequest{}
	reqBody.Amount.Value = "50.00"
	reqBody.Amount.Currency = "RUB"
	reqBody.Confirmation.Type = "redirect"
	reqBody.Confirmation.ReturnURL = "https://t.me/Twitchmanannouncer_bot"
	reqBody.Description = fmt.Sprintf("Pro подписка TwitchAnnouncer для пользователя %d", telegramID)
	reqBody.Metadata = map[string]string{"telegram_id": fmt.Sprintf("%d", telegramID)}

	jsonData, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", "https://api.yookassa.ru/v3/payments", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+os.Getenv("YOOKASSA_AUTH")) // базовая авторизация (shopId:secret) в base64

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var respData YooKassaPaymentResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return "", err
	}

	if respData.Confirmation.URL == "" {
		return "", fmt.Errorf("не удалось получить ссылку на оплату")
	}

	return respData.Confirmation.URL, nil
}
