package game

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

// GameType represents a supported game type
type GameType string

const (
	GameTypeLoL GameType = "lol"
	// Future games can be added here:
	// GameTypeValorant GameType = "valorant"
	// GameTypeTFT      GameType = "tft"
)

// PlayerInfo contains common player identification information
type PlayerInfo struct {
	ID          string   // Unique player identifier (PUUID, Steam ID, etc.)
	DisplayName string   // Human-readable display name
	GameType    GameType // Which game this player is tracked for
}

// MatchInfo contains common match information
type MatchInfo struct {
	MatchID     string      // Unique match identifier
	GameType    GameType    // Which game this match is from
	EndTime     int64       // Unix timestamp when match ended
	RawData     interface{} // Game-specific match data
	PlayerData  interface{} // Game-specific player data from the match
}

// Tracker defines the interface that all game trackers must implement
type Tracker interface {
	// Name returns the human-readable name of the game
	Name() string

	// Type returns the game type identifier
	Type() GameType

	// Description returns a brief description of the game
	Description() string

	// ValidatePlayerID validates the player identifier format
	// Returns an error with a helpful message if invalid
	ValidatePlayerID(input string) error

	// ResolvePlayer looks up player information from the game's API
	// The input format depends on the game (e.g., "Name#Tag" for Riot games)
	ResolvePlayer(ctx context.Context, input string) (*PlayerInfo, error)

	// GetLatestMatchID retrieves the most recent match ID for a player
	GetLatestMatchID(ctx context.Context, playerID string) (string, error)

	// GetMatchDetails fetches detailed match information
	GetMatchDetails(ctx context.Context, matchID string) (*MatchInfo, error)

	// FormatNotification creates a Discord embed for a match notification
	FormatNotification(playerName string, match *MatchInfo) *discordgo.MessageEmbed
}
