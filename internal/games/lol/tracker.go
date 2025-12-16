package lol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/game"
	"github.com/flor3z/discord-bot/internal/riot"
)

// Tracker implements game.Tracker for League of Legends
type Tracker struct {
	client *riot.Client
}

// NewTracker creates a new LoL tracker
func NewTracker(apiKey string) *Tracker {
	return &Tracker{
		client: riot.NewClient(apiKey),
	}
}

// Name returns the human-readable name of the game
func (t *Tracker) Name() string {
	return "League of Legends"
}

// Type returns the game type identifier
func (t *Tracker) Type() game.GameType {
	return game.GameTypeLoL
}

// Description returns a brief description of the game
func (t *Tracker) Description() string {
	return "Track match results for League of Legends summoners"
}

// ValidatePlayerID validates the Riot ID format
func (t *Tracker) ValidatePlayerID(input string) error {
	parts := strings.Split(input, "#")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format: must be GameName#TagLine (e.g., Faker#KR1)")
	}

	gameName := strings.TrimSpace(parts[0])
	tagLine := strings.TrimSpace(parts[1])

	if gameName == "" || tagLine == "" {
		return fmt.Errorf("game name and tag line cannot be empty")
	}

	return nil
}

// ResolvePlayer looks up player information from Riot API
func (t *Tracker) ResolvePlayer(ctx context.Context, input string) (*game.PlayerInfo, error) {
	parts := strings.Split(input, "#")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Riot ID format")
	}

	gameName := strings.TrimSpace(parts[0])
	tagLine := strings.TrimSpace(parts[1])

	account, err := t.client.GetAccountByRiotID(ctx, gameName, tagLine)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve player: %w", err)
	}

	return &game.PlayerInfo{
		ID:          account.PUUID,
		DisplayName: fmt.Sprintf("%s#%s", account.GameName, account.TagLine),
		GameType:    game.GameTypeLoL,
	}, nil
}

// GetLatestMatchID retrieves the most recent match ID for a player
func (t *Tracker) GetLatestMatchID(ctx context.Context, playerID string) (string, error) {
	matchIDs, err := t.client.GetMatchIDsByPUUID(ctx, playerID, 1)
	if err != nil {
		return "", err
	}

	if len(matchIDs) == 0 {
		return "", nil
	}

	return matchIDs[0], nil
}

// GetMatchDetails fetches detailed match information
func (t *Tracker) GetMatchDetails(ctx context.Context, matchID string) (*game.MatchInfo, error) {
	match, err := t.client.GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}

	return &game.MatchInfo{
		MatchID:  match.Metadata.MatchID,
		GameType: game.GameTypeLoL,
		EndTime:  match.Info.GameEndTimestamp,
		RawData:  match,
	}, nil
}

// FormatNotification creates a Discord embed for a match notification
func (t *Tracker) FormatNotification(playerName string, matchInfo *game.MatchInfo) *discordgo.MessageEmbed {
	match, ok := matchInfo.RawData.(*riot.Match)
	if !ok {
		return &discordgo.MessageEmbed{
			Title:       "Match Result",
			Description: "Error formatting match data",
			Color:       0xFF0000,
		}
	}

	// Find the player in the match
	var participant *riot.Participant
	for i := range match.Info.Participants {
		p := &match.Info.Participants[i]
		displayName := fmt.Sprintf("%s#%s", p.RiotIdGameName, p.RiotIdTagline)
		if displayName == playerName {
			participant = p
			break
		}
	}

	// If not found by display name, this might be a data format issue
	if participant == nil {
		return &discordgo.MessageEmbed{
			Title:       "Match Result",
			Description: fmt.Sprintf("Could not find player %s in match data", playerName),
			Color:       0xFF0000,
		}
	}

	return createMatchEmbed(playerName, match, participant)
}

// FindParticipantByPUUID finds a participant in match data by PUUID
func (t *Tracker) FindParticipantByPUUID(matchInfo *game.MatchInfo, puuid string) *riot.Participant {
	match, ok := matchInfo.RawData.(*riot.Match)
	if !ok {
		return nil
	}
	return match.FindParticipant(puuid)
}

// FormatNotificationByPUUID creates a Discord embed using PUUID to find the player
func (t *Tracker) FormatNotificationByPUUID(playerName string, matchInfo *game.MatchInfo, puuid string) *discordgo.MessageEmbed {
	match, ok := matchInfo.RawData.(*riot.Match)
	if !ok {
		return &discordgo.MessageEmbed{
			Title:       "Match Result",
			Description: "Error formatting match data",
			Color:       0xFF0000,
		}
	}

	participant := match.FindParticipant(puuid)
	if participant == nil {
		return &discordgo.MessageEmbed{
			Title:       "Match Result",
			Description: fmt.Sprintf("Could not find player in match data"),
			Color:       0xFF0000,
		}
	}

	return createMatchEmbed(playerName, match, participant)
}

// createMatchEmbed creates a Discord embed for match notification
func createMatchEmbed(playerName string, match *riot.Match, p *riot.Participant) *discordgo.MessageEmbed {
	// Determine color based on win/loss
	color := 0xE74C3C // Red for loss
	resultText := "Defeat"
	if p.Win {
		color = 0x2ECC71 // Green for win
		resultText = "Victory"
	}

	// Calculate KDA
	kda := float64(p.Kills+p.Assists) / float64(max(p.Deaths, 1))
	cs := p.TotalMinionsKilled + p.NeutralMinionsKilled
	gameDurationMin := float64(match.Info.GameDuration) / 60.0
	csPerMin := float64(cs) / gameDurationMin

	// Format duration
	minutes := match.Info.GameDuration / 60
	seconds := match.Info.GameDuration % 60
	durationStr := fmt.Sprintf("%d:%02d", minutes, seconds)

	// Queue name
	queueName := riot.GetQueueName(match.Info.QueueID)

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title: resultText,
		Color: color,
		Author: &discordgo.MessageEmbedAuthor{
			Name: playerName,
		},
		Description: fmt.Sprintf("**%s** | %s", p.ChampionName, queueName),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "KDA",
				Value:  fmt.Sprintf("%d / %d / %d (%.2f)", p.Kills, p.Deaths, p.Assists, kda),
				Inline: true,
			},
			{
				Name:   "CS",
				Value:  fmt.Sprintf("%d (%.1f/min)", cs, csPerMin),
				Inline: true,
			},
			{
				Name:   "Damage",
				Value:  formatNumber(p.TotalDamageDealtToChampions),
				Inline: true,
			},
			{
				Name:   "Gold",
				Value:  formatNumber(p.GoldEarned),
				Inline: true,
			},
			{
				Name:   "Vision",
				Value:  fmt.Sprintf("%d", p.VisionScore),
				Inline: true,
			},
			{
				Name:   "Duration",
				Value:  durationStr,
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Match ID: %s", match.Metadata.MatchID),
		},
		Timestamp: time.UnixMilli(match.Info.GameEndTimestamp).Format(time.RFC3339),
	}

	return embed
}

// formatNumber formats large numbers with commas
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
