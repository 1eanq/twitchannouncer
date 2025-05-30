package yookassa

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Client struct {
	ShopID    string
	SecretKey string
	HTTP      *http.Client
}

func NewClient() *Client {
	shopID := os.Getenv("YOOKASSA_SHOP_ID")
	secretKey := os.Getenv("YOOKASSA_SECRET_KEY")

	fmt.Println("Shop ID:", shopID)
	fmt.Println("Secret Key:", secretKey)

	return &Client{
		ShopID:    os.Getenv("YOOKASSA_SHOP_ID"),
		SecretKey: os.Getenv("YOOKASSA_SECRET_KEY"),
		HTTP:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) NewRequest(method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	authRaw := c.ShopID + ":" + c.SecretKey
	authEncoded := base64.StdEncoding.EncodeToString([]byte(authRaw))

	req.Header.Set("Authorization", "Basic "+authEncoded)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", generateIdempotenceKey())
	return req, nil
}

func generateIdempotenceKey() string {
	return time.Now().Format("20060102150405")
}
