package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func RefreshTwitchToken(cfg *Config, configFile string) error {
	if time.Now().Unix() < cfg.TwitchOAuthExpires-60 {
		return nil
	}

	form := url.Values{}
	form.Add("client_id", cfg.TwitchClientID)
	form.Add("client_secret", cfg.TwitchClientSecret)
	form.Add("grant_type", "client_credentials")

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", form)
	if err != nil {
		return fmt.Errorf("ошибка получения токена: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("ошибка разбора ответа: %v", err)
	}

	cfg.TwitchOAuthToken = result.AccessToken
	cfg.TwitchOAuthExpires = time.Now().Unix() + result.ExpiresIn
	SaveConfig(configFile, *cfg)

	fmt.Println("Токен Twitch обновлён")
	return nil
}
