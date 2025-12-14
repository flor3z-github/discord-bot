package storage

import "time"

// Summoner represents a tracked League of Legends summoner
type Summoner struct {
	ID          int64
	PUUID       string
	RiotID      string // GameName#TagLine
	Region      string
	LastMatchID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GuildSettings stores per-server configuration
type GuildSettings struct {
	GuildID               string
	NotificationChannelID string
	CreatedAt             time.Time
}

// Subscription links a summoner to a Discord guild
type Subscription struct {
	ID           int64
	SummonerID   int64
	GuildID      string
	RegisteredBy string // Discord user ID
	CreatedAt    time.Time
}
