package storage

import "time"

// Summoner represents a tracked game player
type Summoner struct {
	ID          int64
	PUUID       string    // Unique player identifier (PUUID, Steam ID, etc.)
	RiotID      string    // Display name (GameName#TagLine for Riot games)
	GameType    string    // Game type identifier (lol, valorant, tft, etc.)
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
