package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"twitchannouncer/internal/config"
	"twitchannouncer/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Monitor struct {
	bot *tgbotapi.BotAPI
	db  *database.DB
	cfg config.Config
}

type StreamInfo struct {
	Title       string
	ViewerCount int
	GameName    string
}

func NewMonitor(bot *tgbotapi.BotAPI, db *database.DB, cfg config.Config) *Monitor {
	return &Monitor{
		bot: bot,
		db:  db,
		cfg: cfg,
	}
}

func (m *Monitor) Start(ctx context.Context, duration time.Duration) {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.checkAllStreams()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *Monitor) checkAllStreams() {
	usernames, err := m.db.GetAllTwitchUsernames()
	if err != nil {
		log.Printf("Ошибка при получении твич-юзеров: %v", err)
		return
	}

	for _, username := range usernames {
		isLive, info := m.checkStreamStatus(username)

		streamData, err := m.db.GetStreamData(username)
		if err != nil {
			log.Printf("Ошибка получения данных о стриме для %s: %v", username, err)
			continue
		}

		channels, err := m.db.GetAllChannelsForUser(username)
		if err != nil {
			log.Printf("Ошибка при получении каналов для %s: %v", username, err)
			continue
		}

		if isLive && (!streamData.Checked || !streamData.Live) {
			proChannels := []int64{}
			regularChannels := []int64{}

			for _, chID := range channels {
				isPro, err := m.db.IsUserPro(chID)
				if err != nil {
					log.Printf("Ошибка проверки Pro статуса для %d: %v", chID, err)
					continue
				}
				if isPro {
					proChannels = append(proChannels, chID)
				} else {
					regularChannels = append(regularChannels, chID)
				}
			}

			var wg sync.WaitGroup

			for _, chID := range proChannels {
				wg.Add(1)
				go func(chID int64) {
					defer wg.Done()
					msg := tgbotapi.NewMessage(chID, fmt.Sprintf(
						"🔴 *%s* начал стрим!\n📝 *Название:* %s\n🎮 *Игра:* %s\n👉 https://twitch.tv/%s",
						username, info.Title, info.GameName, username))
					msg.ParseMode = "MarkdownV2"

					sentMsg, err := m.bot.Send(msg)
					if err != nil {
						log.Printf("Ошибка отправки Pro-сообщения: %v", err)
						return
					}

					err = m.db.UpdateStreamStatus(username, true, true, sentMsg.MessageID)
					if err != nil {
						log.Printf("Ошибка обновления статуса стрима: %v", err)
					}
				}(chID)
			}

			wg.Wait()

			for _, chID := range regularChannels {
				wg.Add(1)
				go func(chID int64) {
					defer wg.Done()
					msg := tgbotapi.NewMessage(chID, fmt.Sprintf(
						"🔴 *%s* начал стрим!\n📝 *Название:* %s\n🎮 *Игра:* %s\n👉 https://twitch.tv/%s\n\nОтправлено с помощью [Twitchmanannouncer_bot](https://t.me/Twitchmanannouncer_bot)",
						username, info.Title, info.GameName, username))
					msg.ParseMode = "MarkdownV2"

					sentMsg, err := m.bot.Send(msg)
					if err != nil {
						log.Printf("Ошибка отправки обычного сообщения: %v", err)
						return
					}

					err = m.db.UpdateStreamStatus(username, true, true, sentMsg.MessageID)
					if err != nil {
						log.Printf("Ошибка обновления статуса стрима: %v", err)
					}
				}(chID)
			}

			wg.Wait()
		}

		if !isLive && streamData.Checked && streamData.Live {
			for _, chID := range channels {
				del := tgbotapi.NewDeleteMessage(chID, streamData.LatestMessageID)
				_, err := m.bot.Request(del)
				if err != nil {
					log.Printf("Ошибка при удалении сообщения: %v", err)
				}
			}

			err = m.db.UpdateStreamStatus(username, false, true, 0)
			if err != nil {
				log.Printf("Ошибка обновления статуса стрима: %v", err)
			}
		}
	}
}

func (m *Monitor) checkStreamStatus(username string) (bool, StreamInfo) {
	url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_login=%s", username)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Client-ID", m.cfg.TwitchClientID)
	req.Header.Set("Authorization", "Bearer "+m.cfg.TwitchOAuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Ошибка запроса к Twitch API: %v", err)
		return false, StreamInfo{}
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Type        string `json:"type"`
			Title       string `json:"title"`
			ViewerCount int    `json:"viewer_count"`
			GameName    string `json:"game_name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Ошибка при декодировании ответа Twitch API: %v", err)
		return false, StreamInfo{}
	}

	if len(result.Data) == 0 {
		return false, StreamInfo{}
	}

	stream := result.Data[0]
	return true, StreamInfo{
		Title:       stream.Title,
		ViewerCount: stream.ViewerCount,
		GameName:    stream.GameName,
	}
}
