package database

type Data struct {
	UserID           int64
	TelegramUsername string
	ChanelID         int64
	TwitchUsername   string
	IsSent           bool
	LatestMessageID  int
}
