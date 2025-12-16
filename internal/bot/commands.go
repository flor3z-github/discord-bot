package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/game"
	"github.com/flor3z/discord-bot/internal/storage"
)

// buildGameChoices creates the game selection choices for slash commands
func (b *Bot) buildGameChoices() []*discordgo.ApplicationCommandOptionChoice {
	games := b.registry.List()
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(games))
	for i, g := range games {
		choices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  g.Name,
			Value: string(g.Type),
		}
	}
	return choices
}

// Slash command definitions
func (b *Bot) getCommandDefinitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "등록",
			Description: "플레이어를 등록하여 경기 기록을 추적합니다",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "게임",
					Description: "추적할 게임 (예: lol)",
					Required:    true,
					Choices:     b.buildGameChoices(),
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "플레이어",
					Description: "플레이어 ID (예: Faker#KR1)",
					Required:    true,
				},
			},
		},
		{
			Name:        "해제",
			Description: "플레이어의 경기 기록 추적을 중지합니다",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "게임",
					Description: "게임 (예: lol)",
					Required:    true,
					Choices:     b.buildGameChoices(),
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "플레이어",
					Description: "플레이어 ID (예: Faker#KR1)",
					Required:    true,
				},
			},
		},
		{
			Name:        "목록",
			Description: "이 서버에 등록된 모든 플레이어 목록",
		},
		{
			Name:        "채널설정",
			Description: "경기 알림을 받을 채널 설정",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "채널",
					Description: "알림을 보낼 채널",
					Required:    true,
					ChannelTypes: []discordgo.ChannelType{
						discordgo.ChannelTypeGuildText,
					},
				},
			},
		},
		{
			Name:        "게임목록",
			Description: "경기 추적이 가능한 게임 목록",
		},
	}
}

// registerCommands registers all slash commands with Discord
func (b *Bot) registerCommands() error {
	slog.Info("Registering slash commands")

	commandDefinitions := b.getCommandDefinitions()
	registeredCommands := make([]*discordgo.ApplicationCommand, 0, len(commandDefinitions))

	for _, cmd := range commandDefinitions {
		registered, err := b.session.ApplicationCommandCreate(
			b.session.State.User.ID,
			"", // Empty string = global command
			cmd,
		)
		if err != nil {
			return fmt.Errorf("failed to register command %s: %w", cmd.Name, err)
		}
		registeredCommands = append(registeredCommands, registered)
		slog.Debug("Registered command", "name", cmd.Name)
	}

	b.commands = registeredCommands
	slog.Info("Slash commands registered", "count", len(registeredCommands))
	return nil
}

// removeCommands removes all registered slash commands
func (b *Bot) removeCommands() {
	for _, cmd := range b.commands {
		err := b.session.ApplicationCommandDelete(b.session.State.User.ID, "", cmd.ID)
		if err != nil {
			slog.Error("Failed to remove command", "name", cmd.Name, "error", err)
		}
	}
}

// handleRegister handles the /register command
func (b *Bot) handleRegister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	gameType := options[0].StringValue()
	playerID := options[1].StringValue()

	// Respond immediately to avoid timeout
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Get the tracker for this game
	tracker, err := b.registry.Get(game.GameType(gameType))
	if err != nil {
		b.editResponse(s, i, fmt.Sprintf("알 수 없는 게임: `%s`. `/게임목록` 명령어로 지원되는 게임을 확인하세요.", gameType))
		return
	}

	// Validate player ID format
	if err := tracker.ValidatePlayerID(playerID); err != nil {
		b.editResponse(s, i, fmt.Sprintf("잘못된 플레이어 ID 형식: %s", err.Error()))
		return
	}

	// Look up player from game API
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	playerInfo, err := tracker.ResolvePlayer(ctx, playerID)
	if err != nil {
		slog.Error("Failed to look up player", "playerID", playerID, "error", err)
		b.editResponse(s, i, fmt.Sprintf("플레이어 `%s`를 찾을 수 없습니다. ID를 확인하고 다시 시도해주세요.", playerID))
		return
	}

	// Get initial match ID
	lastMatchID, err := tracker.GetLatestMatchID(ctx, playerInfo.ID)
	if err != nil {
		slog.Warn("Failed to get initial match history", "playerID", playerInfo.ID, "error", err)
		// Continue without last match ID - will be set on first poll
	}

	// Store summoner
	summoner := &storage.Summoner{
		PUUID:       playerInfo.ID,
		RiotID:      playerInfo.DisplayName,
		GameType:    string(playerInfo.GameType),
		Region:      "KR", // Default to KR for now
		LastMatchID: lastMatchID,
	}

	if err := b.repo.CreateSummoner(summoner); err != nil {
		// Check if already exists
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			// Try to get existing summoner and add subscription
			existing, _ := b.repo.GetSummonerByPUUIDAndGame(playerInfo.ID, string(playerInfo.GameType))
			if existing != nil {
				summoner = existing
			} else {
				b.editResponse(s, i, fmt.Sprintf("플레이어 `%s`는 이미 %s에 등록되어 있습니다.", summoner.RiotID, tracker.Name()))
				return
			}
		} else {
			slog.Error("Failed to save summoner", "error", err)
			b.editResponse(s, i, "플레이어 등록에 실패했습니다. 다시 시도해주세요.")
			return
		}
	}

	// Create subscription for this guild
	sub := &storage.Subscription{
		SummonerID:   summoner.ID,
		GuildID:      i.GuildID,
		RegisteredBy: i.Member.User.ID,
	}

	if err := b.repo.CreateSubscription(sub); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			b.editResponse(s, i, fmt.Sprintf("플레이어 `%s`는 이미 이 서버에서 %s 추적 중입니다.", summoner.RiotID, tracker.Name()))
			return
		}
		slog.Error("Failed to create subscription", "error", err)
		b.editResponse(s, i, "플레이어는 저장되었으나 구독 생성에 실패했습니다.")
		return
	}

	b.editResponse(s, i, fmt.Sprintf("`%s`를 %s 경기 추적에 성공적으로 등록했습니다!", summoner.RiotID, tracker.Name()))
}

// handleUnregister handles the /unregister command
func (b *Bot) handleUnregister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	gameType := options[0].StringValue()
	playerID := options[1].StringValue()

	// Get the tracker for this game
	tracker, err := b.registry.Get(game.GameType(gameType))
	if err != nil {
		respondWithMessage(s, i, fmt.Sprintf("알 수 없는 게임: `%s`. `/게임목록` 명령어로 지원되는 게임을 확인하세요.", gameType))
		return
	}

	// Validate and normalize player ID
	if err := tracker.ValidatePlayerID(playerID); err != nil {
		respondWithMessage(s, i, fmt.Sprintf("잘못된 플레이어 ID 형식: %s", err.Error()))
		return
	}

	// Find summoner
	summoner, err := b.repo.GetSummonerByRiotIDAndGame(playerID, gameType)
	if err != nil {
		respondWithMessage(s, i, fmt.Sprintf("플레이어 `%s`는 %s에 등록되어 있지 않습니다.", playerID, tracker.Name()))
		return
	}

	// Delete subscription for this guild
	if err := b.repo.DeleteSubscription(summoner.ID, i.GuildID); err != nil {
		slog.Error("Failed to delete subscription", "error", err)
		respondWithMessage(s, i, "플레이어 등록 해제에 실패했습니다. 다시 시도해주세요.")
		return
	}

	respondWithMessage(s, i, fmt.Sprintf("`%s`를 %s 추적에서 성공적으로 해제했습니다.", playerID, tracker.Name()))
}

// handleList handles the /list command
func (b *Bot) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	summoners, err := b.repo.GetSummonersByGuild(i.GuildID)
	if err != nil {
		slog.Error("Failed to get summoners", "error", err)
		respondWithMessage(s, i, "플레이어 목록을 가져오는 데 실패했습니다.")
		return
	}

	if len(summoners) == 0 {
		respondWithMessage(s, i, "이 서버에 등록된 플레이어가 없습니다.\n`/등록` 명령어로 추가하세요!\n`/게임목록` 명령어로 지원되는 게임을 확인하세요.")
		return
	}

	// Group summoners by game type
	byGame := make(map[string][]*storage.Summoner)
	for _, summoner := range summoners {
		gameType := summoner.GameType
		if gameType == "" {
			gameType = "lol" // Legacy default
		}
		byGame[gameType] = append(byGame[gameType], summoner)
	}

	// Build list
	var sb strings.Builder
	sb.WriteString("**등록된 플레이어:**\n\n")

	for gameType, players := range byGame {
		// Get game name
		gameName := gameType
		if tracker, err := b.registry.Get(game.GameType(gameType)); err == nil {
			gameName = tracker.Name()
		}

		sb.WriteString(fmt.Sprintf("**%s:**\n", gameName))
		for idx, summoner := range players {
			sb.WriteString(fmt.Sprintf("  %d. `%s`\n", idx+1, summoner.RiotID))
		}
		sb.WriteString("\n")
	}

	respondWithMessage(s, i, sb.String())
}

// handleSetChannel handles the /setchannel command
func (b *Bot) handleSetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channel := i.ApplicationCommandData().Options[0].ChannelValue(s)

	settings := &storage.GuildSettings{
		GuildID:               i.GuildID,
		NotificationChannelID: channel.ID,
	}

	if err := b.repo.UpsertGuildSettings(settings); err != nil {
		slog.Error("Failed to save guild settings", "error", err)
		respondWithMessage(s, i, "알림 채널 설정에 실패했습니다. 다시 시도해주세요.")
		return
	}

	respondWithMessage(s, i, fmt.Sprintf("경기 알림이 <#%s> 채널로 전송됩니다", channel.ID))
}

// handleGames handles the /games command
func (b *Bot) handleGames(s *discordgo.Session, i *discordgo.InteractionCreate) {
	games := b.registry.List()

	if len(games) == 0 {
		respondWithMessage(s, i, "현재 지원되는 게임이 없습니다.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**지원되는 게임:**\n\n")

	for _, g := range games {
		sb.WriteString(fmt.Sprintf("**%s** (`%s`)\n", g.Name, g.Type))
		sb.WriteString(fmt.Sprintf("  %s\n\n", g.Description))
	}

	sb.WriteString("`/등록 게임:<게임> 플레이어:<ID>` 명령어로 추적을 시작하세요!")

	respondWithMessage(s, i, sb.String())
}

// Helper functions

func respondWithMessage(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

func (b *Bot) editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}
