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
	return "리그 오브 레전드"
}

// Type returns the game type identifier
func (t *Tracker) Type() game.GameType {
	return game.GameTypeLoL
}

// Description returns a brief description of the game
func (t *Tracker) Description() string {
	return "리그 오브 레전드 소환사의 경기 결과 추적"
}

// ValidatePlayerID validates the Riot ID format
func (t *Tracker) ValidatePlayerID(input string) error {
	parts := strings.Split(input, "#")
	if len(parts) != 2 {
		return fmt.Errorf("잘못된 형식: 소환사명#태그 형식이어야 합니다 (예: Faker#KR1)")
	}

	gameName := strings.TrimSpace(parts[0])
	tagLine := strings.TrimSpace(parts[1])

	if gameName == "" || tagLine == "" {
		return fmt.Errorf("소환사명과 태그는 비워둘 수 없습니다")
	}

	return nil
}

// ResolvePlayer looks up player information from Riot API
func (t *Tracker) ResolvePlayer(ctx context.Context, input string) (*game.PlayerInfo, error) {
	parts := strings.Split(input, "#")
	if len(parts) != 2 {
		return nil, fmt.Errorf("잘못된 Riot ID 형식")
	}

	gameName := strings.TrimSpace(parts[0])
	tagLine := strings.TrimSpace(parts[1])

	account, err := t.client.GetAccountByRiotID(ctx, gameName, tagLine)
	if err != nil {
		return nil, fmt.Errorf("플레이어를 찾을 수 없습니다: %w", err)
	}

	return &game.PlayerInfo{
		ID:          account.PUUID,
		DisplayName: fmt.Sprintf("%s#%s", account.GameName, account.TagLine),
		GameType:    game.GameTypeLoL,
	}, nil
}

// GetCurrentState returns the latest match ID for change detection
func (t *Tracker) GetCurrentState(ctx context.Context, playerID string) (string, error) {
	matchIDs, err := t.client.GetMatchIDsByPUUID(ctx, playerID, 1)
	if err != nil {
		return "", err
	}

	if len(matchIDs) == 0 {
		return "", nil
	}

	return matchIDs[0], nil
}

// CreateNotification fetches match details and creates a Discord embed
func (t *Tracker) CreateNotification(ctx context.Context, playerID, playerName, stateID string) (*discordgo.MessageEmbed, error) {
	// stateID is the match ID for LoL
	match, err := t.client.GetMatch(ctx, stateID)
	if err != nil {
		return nil, fmt.Errorf("경기 정보를 가져올 수 없습니다: %w", err)
	}

	// Find the player in the match by PUUID
	participant := match.FindParticipant(playerID)
	if participant == nil {
		return &discordgo.MessageEmbed{
			Title:       "경기 결과",
			Description: "경기 데이터에서 플레이어를 찾을 수 없습니다",
			Color:       0xFF0000,
		}, nil
	}

	return createMatchEmbed(playerName, match, participant), nil
}

// createMatchEmbed creates a Discord embed for match notification
func createMatchEmbed(playerName string, match *riot.Match, p *riot.Participant) *discordgo.MessageEmbed {
	// Determine color based on win/loss
	color := 0xE74C3C // Red for loss
	resultText := "패배"
	if p.Win {
		color = 0x2ECC71 // Green for win
		resultText = "승리"
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
				Name:   "피해량",
				Value:  formatNumber(p.TotalDamageDealtToChampions),
				Inline: true,
			},
			{
				Name:   "골드",
				Value:  formatNumber(p.GoldEarned),
				Inline: true,
			},
			{
				Name:   "시야 점수",
				Value:  fmt.Sprintf("%d", p.VisionScore),
				Inline: true,
			},
			{
				Name:   "경기 시간",
				Value:  durationStr,
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("경기 ID: %s", match.Metadata.MatchID),
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
