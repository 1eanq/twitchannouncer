package config

import (
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

type Config struct {
	TelegramToken      string `yaml:"telegram_token"`
	TwitchClientID     string `yaml:"twitch_client_id"`
	TwitchClientSecret string `yaml:"twitch_client_secret"`
	TwitchOAuthToken   string `yaml:"twitch_oauth_token"`
	TwitchOAuthExpires int64  `yaml:"twitch_oauth_expires"`
	DatabaseUser       string `yaml:"database_user"`
	DatabasePassword   string `yaml:"database_password"`
	DatabaseHost       string `yaml:"database_host"`
	DatabasePort       string `yaml:"database_port"`
	DatabaseName       string `yaml:"database_name"`
}

func LoadConfig(filename string) Config {
	var cfg Config
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Ошибка при открытии %s: %v", filename, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		log.Fatalf("Ошибка при чтении %s: %v", filename, err)
	}
	return cfg
}

func SaveConfig(filename string, cfg Config) {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Не удалось сохранить %s: %v", filename, err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(&cfg); err != nil {
		log.Fatalf("Ошибка при сохранении %s: %v", err)
	}
}
