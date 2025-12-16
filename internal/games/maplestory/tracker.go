package maplestory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/game"
	"github.com/flor3z/discord-bot/internal/nexon"
)

// Tracker implements game.Tracker for MapleStory
type Tracker struct {
	client *nexon.Client
}

// NewTracker creates a new MapleStory tracker
func NewTracker(apiKey string) *Tracker {
	return &Tracker{
		client: nexon.NewClient(apiKey),
	}
}

// Name returns the human-readable name of the game
func (t *Tracker) Name() string {
	return "메이플스토리"
}

// Type returns the game type identifier
func (t *Tracker) Type() game.GameType {
	return game.GameTypeMaplestory
}

// Description returns a brief description of the game
func (t *Tracker) Description() string {
	return "메이플스토리 캐릭터 레벨/경험치 추적"
}

// ValidatePlayerID validates the character name format
func (t *Tracker) ValidatePlayerID(input string) error {
	name := strings.TrimSpace(input)
	if name == "" {
		return fmt.Errorf("캐릭터 이름이 비어있습니다")
	}
	if len(name) > 12 {
		return fmt.Errorf("캐릭터 이름이 너무 깁니다 (최대 12자)")
	}
	return nil
}

// ResolvePlayer looks up player information from Nexon API
func (t *Tracker) ResolvePlayer(ctx context.Context, input string) (*game.PlayerInfo, error) {
	characterName := strings.TrimSpace(input)

	ocidResp, err := t.client.GetCharacterOCID(ctx, characterName)
	if err != nil {
		return nil, fmt.Errorf("캐릭터를 찾을 수 없습니다: %w", err)
	}

	// Get basic info to confirm the character exists and get the exact name
	basicInfo, err := t.client.GetCharacterBasic(ctx, ocidResp.OCID)
	if err != nil {
		return nil, fmt.Errorf("캐릭터 정보를 가져올 수 없습니다: %w", err)
	}

	return &game.PlayerInfo{
		ID:          ocidResp.OCID,
		DisplayName: basicInfo.CharacterName,
		GameType:    game.GameTypeMaplestory,
	}, nil
}

// GetCurrentState returns a state hash based on level and exp for change detection
func (t *Tracker) GetCurrentState(ctx context.Context, playerID string) (string, error) {
	basicInfo, err := t.client.GetCharacterBasic(ctx, playerID)
	if err != nil {
		return "", err
	}

	// Return state hash: lv:{level}:exp:{exp}
	// This changes when level or exp changes
	return fmt.Sprintf("lv:%d:exp:%d", basicInfo.CharacterLevel, basicInfo.CharacterExp), nil
}

// CreateNotification fetches fresh character data and creates a Discord embed
func (t *Tracker) CreateNotification(ctx context.Context, playerID, playerName, stateID string) (*discordgo.MessageEmbed, error) {
	// Fetch fresh character data using the OCID (playerID)
	basicInfo, err := t.client.GetCharacterBasic(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("캐릭터 정보를 가져올 수 없습니다: %w", err)
	}

	embed := &discordgo.MessageEmbed{
		Title: "📊 메이플스토리 캐릭터 상태",
		Color: 0xFF9900, // Orange color for MapleStory
		Author: &discordgo.MessageEmbedAuthor{
			Name: playerName,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "레벨",
				Value:  fmt.Sprintf("%d", basicInfo.CharacterLevel),
				Inline: true,
			},
			{
				Name:   "경험치",
				Value:  fmt.Sprintf("%s%%", basicInfo.CharacterExpRate),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "메이플스토리",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return embed, nil
}
