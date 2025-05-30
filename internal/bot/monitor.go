package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
				m.Monitoring()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *Monitor) Monitoring() {
	subs, err := m.db.GetAllSubscriptions()
	if err != nil {
		log.Println(err)
	}

	for _, sub := range subs {
		isLive, info := m.checkStreamStatus(sub.TwitchUsername)

		streamData, err := m.db.GetStreamData(sub.TwitchUsername)
		if err != nil {
			log.Printf("Ошибка при получении твич-юзеров: %v", err)
			return
		}

		if isLive && (!streamData.Checked || !streamData.Live) {
			isPro, err := m.db.IsUserPro(sub.UserID)
			if err != nil {
				log.Println(err)
			}
			if isPro {
				text := fmt.Sprintf(
					"🔴 *%s* начал стрим!\n📝 *Название:* %s\n🎮 *Игра:* %s\n👉 https://twitch.tv/%s",
					escapeMarkdown(sub.TwitchUsername),
					escapeMarkdown(info.Title),
					escapeMarkdown(info.GameName),
					escapeMarkdown(sub.TwitchUsername),
				)

				msg := tgbotapi.NewMessage(sub.ChannelID, text)
				msg.ParseMode = "MarkdownV2"

				sentMsg, err := m.bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
					continue
				}
				log.Printf("Сообщение успешно отправлено. %s", sentMsg.Text)

				err = m.db.UpdateStreamStatus(sub.TwitchUsername, true, true, sentMsg.MessageID)
				if err != nil {
					log.Printf("Ошибка обновления статуса стрима: %v", err)
				}
			} else {
				text := fmt.Sprintf(
					"🔴 *%s* начал стрим!\n📝 *Название:* %s\n🎮 *Игра:* %s\n👉 https://twitch.tv/%s\n\nОтправлено с помощью https://t.me/Twitchmanannouncer_bot",
					sub.TwitchUsername,
					info.Title,
					info.GameName,
					sub.TwitchUsername,
				)

				msg := tgbotapi.NewMessage(sub.ChannelID, escapeMarkdown(text))
				msg.ParseMode = "MarkdownV2"

				sentMsg, err := m.bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
					continue
				}
				log.Printf("Сообщение успешно отправлено. %s", sentMsg.Text)

				err = m.db.UpdateStreamStatus(sub.TwitchUsername, true, true, sentMsg.MessageID)
				if err != nil {
					log.Printf("Ошибка обновления статуса стрима: %v", err)
				}
			}
		}

		if !isLive && streamData.Checked && streamData.Live {
			del := tgbotapi.NewDeleteMessage(sub.ChannelID, sub.LatestMessageID)
			_, err := m.bot.Request(del)
			if err != nil {
				log.Printf("Ошибка при удалении сообщения: %v", err)
			}
			err = m.db.UpdateStreamStatus(sub.TwitchUsername, false, false, 0)

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

func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		`_`, `\_`,
		`[`, `\[`,
		`]`, `\]`,
		`(`, `\(`,
		`)`, `\)`,
		`~`, `\~`,
		"`", "\\`",
		`>`, `\>`,
		`#`, `\#`,
		`+`, `\+`,
		`-`, `\-`,
		`=`, `\=`,
		`|`, `\|`,
		`{`, `\{`,
		`}`, `\}`,
		`.`, `\.`,
		`!`, `\!`,
	)
	return replacer.Replace(text)
}
