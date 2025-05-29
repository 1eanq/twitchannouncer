package database

import "time"

type UserData struct {
	TelegramID       int64
	TelegramUsername string
	Admin            bool
	expires_at       time.Time
}

type SubscriptionData struct {
	UserID          int64
	ChannelID       int64
	TwitchUsername  string
	LatestMessageID int
	Checked         bool
	Live            bool
	ChannelName     string
}
