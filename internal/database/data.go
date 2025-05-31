package database

import (
	"time"
)

type UserData struct {
	TelegramID       int64
	TelegramUsername string
	Admin            bool
	Expires_at       time.Time
	Email            string
}

type SubscriptionData struct {
	ID              int
	UserID          int64
	ChannelID       int64
	TwitchUsername  string
	LatestMessageID int
	Checked         bool
	Live            bool
	ChannelName     string
}
