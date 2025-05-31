package yookassa

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	Receipt     struct {
		Customer struct {
			Email string `json:"email"`
		} `json:"customer"`
		Items []struct {
			Description string `json:"description"`
			Quantity    string `json:"quantity"`
			Amount      struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			} `json:"amount"`
			VatCode int `json:"vat_code"` // 1 — Без НДС
		} `json:"items"`
	} `json:"receipt"`
}

type YooKassaPaymentResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Confirmation struct {
		Type string `json:"type"`
		URL  string `json:"confirmation_url"`
	} `json:"confirmation"`
}

func (c *Client) CreatePayment(telegramID int64, email string) (string, error) {
	reqBody := YooKassaPaymentRequest{}
	reqBody.Amount.Value = "50.00"
	reqBody.Amount.Currency = "RUB"
	reqBody.Confirmation.Type = "redirect"
	reqBody.Confirmation.ReturnURL = "https://t.me/Twitchmanannouncer_bot"
	reqBody.Description = fmt.Sprintf("Pro подписка TwitchAnnouncer для пользователя %d", telegramID)
	reqBody.Metadata = map[string]string{"telegram_id": fmt.Sprintf("%d", telegramID)}

	// Добавляем чек
	reqBody.Receipt.Customer.Email = email
	reqBody.Receipt.Items = []struct {
		Description string `json:"description"`
		Quantity    string `json:"quantity"`
		Amount      struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"amount"`
		VatCode int `json:"vat_code"`
	}{
		{
			Description: "Pro подписка TwitchAnnouncer",
			Quantity:    "1",
			Amount: struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			}{
				Value:    "50.00",
				Currency: "RUB",
			},
			VatCode: 1, // Без НДС
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, err := c.NewRequest("POST", "https://api.yookassa.ru/v3/payments", jsonData)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка от YooKassa [%d]: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData YooKassaPaymentResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return "", err
	}

	if respData.Confirmation.URL == "" {
		return "", fmt.Errorf("не удалось получить ссылку на оплату")
	}

	log.Printf("Создана платежная ссылка: %s", respData.Confirmation.URL)

	return respData.Confirmation.URL, nil
}
