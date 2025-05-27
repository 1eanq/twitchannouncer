package database

type UserData struct {
	TelegramID       int64
	TelegramUsername string
	TwitchUsername   string
	ChannelID        int64
}

type StreamData struct {
	UserID          int64
	TwitchUsername  string
	Live            bool
	Checked         bool
	LatestMessageID int
}
