package game

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

// GameType represents a supported game type
type GameType string

const (
	GameTypeLoL        GameType = "lol"
	GameTypeMaplestory GameType = "maplestory"
)

// PlayerInfo contains common player identification information
type PlayerInfo struct {
	ID          string   // Unique player identifier (PUUID, OCID, etc.)
	DisplayName string   // Human-readable display name
	GameType    GameType // Which game this player is tracked for
}

// Tracker defines the interface that all game trackers must implement
// This interface is generic enough to support both match-based games (LoL)
// and progression-based games (MapleStory)
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

	// GetCurrentState returns a state identifier for change detection
	// For match-based games: returns latest match ID
	// For progression games: returns a hash of current state (e.g., "lv:275:exp:12345")
	GetCurrentState(ctx context.Context, playerID string) (string, error)

	// CreateNotification creates a Discord embed for a state change notification
	// playerID allows the tracker to fetch fresh data if needed
	// stateID is the new state that triggered the notification
	CreateNotification(ctx context.Context, playerID, playerName, stateID string) (*discordgo.MessageEmbed, error)
}
