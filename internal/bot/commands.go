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
			Name:        "register",
			Description: "Register a player to track match history",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "game",
					Description: "The game to track (e.g., lol)",
					Required:    true,
					Choices:     b.buildGameChoices(),
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "player_id",
					Description: "Player identifier (e.g., Faker#KR1 for LoL)",
					Required:    true,
				},
			},
		},
		{
			Name:        "unregister",
			Description: "Stop tracking a player's match history",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "game",
					Description: "The game (e.g., lol)",
					Required:    true,
					Choices:     b.buildGameChoices(),
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "player_id",
					Description: "Player identifier (e.g., Faker#KR1 for LoL)",
					Required:    true,
				},
			},
		},
		{
			Name:        "list",
			Description: "List all registered players in this server",
		},
		{
			Name:        "setchannel",
			Description: "Set the channel for match notifications",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "channel",
					Description: "The channel to send notifications to",
					Required:    true,
					ChannelTypes: []discordgo.ChannelType{
						discordgo.ChannelTypeGuildText,
					},
				},
			},
		},
		{
			Name:        "games",
			Description: "List all supported games for match tracking",
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
		b.editResponse(s, i, fmt.Sprintf("Unknown game: `%s`. Use `/games` to see supported games.", gameType))
		return
	}

	// Validate player ID format
	if err := tracker.ValidatePlayerID(playerID); err != nil {
		b.editResponse(s, i, fmt.Sprintf("Invalid player ID format: %s", err.Error()))
		return
	}

	// Look up player from game API
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	playerInfo, err := tracker.ResolvePlayer(ctx, playerID)
	if err != nil {
		slog.Error("Failed to look up player", "playerID", playerID, "error", err)
		b.editResponse(s, i, fmt.Sprintf("Could not find player `%s`. Please check the ID and try again.", playerID))
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
				b.editResponse(s, i, fmt.Sprintf("Player `%s` is already registered for %s.", summoner.RiotID, tracker.Name()))
				return
			}
		} else {
			slog.Error("Failed to save summoner", "error", err)
			b.editResponse(s, i, "Failed to register player. Please try again.")
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
			b.editResponse(s, i, fmt.Sprintf("Player `%s` is already being tracked in this server for %s.", summoner.RiotID, tracker.Name()))
			return
		}
		slog.Error("Failed to create subscription", "error", err)
		b.editResponse(s, i, "Player saved but failed to create subscription.")
		return
	}

	b.editResponse(s, i, fmt.Sprintf("Successfully registered `%s` for %s match tracking!", summoner.RiotID, tracker.Name()))
}

// handleUnregister handles the /unregister command
func (b *Bot) handleUnregister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	gameType := options[0].StringValue()
	playerID := options[1].StringValue()

	// Get the tracker for this game
	tracker, err := b.registry.Get(game.GameType(gameType))
	if err != nil {
		respondWithMessage(s, i, fmt.Sprintf("Unknown game: `%s`. Use `/games` to see supported games.", gameType))
		return
	}

	// Validate and normalize player ID
	if err := tracker.ValidatePlayerID(playerID); err != nil {
		respondWithMessage(s, i, fmt.Sprintf("Invalid player ID format: %s", err.Error()))
		return
	}

	// Find summoner
	summoner, err := b.repo.GetSummonerByRiotIDAndGame(playerID, gameType)
	if err != nil {
		respondWithMessage(s, i, fmt.Sprintf("Player `%s` is not registered for %s.", playerID, tracker.Name()))
		return
	}

	// Delete subscription for this guild
	if err := b.repo.DeleteSubscription(summoner.ID, i.GuildID); err != nil {
		slog.Error("Failed to delete subscription", "error", err)
		respondWithMessage(s, i, "Failed to unregister player. Please try again.")
		return
	}

	respondWithMessage(s, i, fmt.Sprintf("Successfully unregistered `%s` from %s tracking.", playerID, tracker.Name()))
}

// handleList handles the /list command
func (b *Bot) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	summoners, err := b.repo.GetSummonersByGuild(i.GuildID)
	if err != nil {
		slog.Error("Failed to get summoners", "error", err)
		respondWithMessage(s, i, "Failed to retrieve player list.")
		return
	}

	if len(summoners) == 0 {
		respondWithMessage(s, i, "No players are registered in this server.\nUse `/register` to add one!\nUse `/games` to see supported games.")
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
	sb.WriteString("**Registered Players:**\n\n")

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
		respondWithMessage(s, i, "Failed to set notification channel. Please try again.")
		return
	}

	respondWithMessage(s, i, fmt.Sprintf("Match notifications will be sent to <#%s>", channel.ID))
}

// handleGames handles the /games command
func (b *Bot) handleGames(s *discordgo.Session, i *discordgo.InteractionCreate) {
	games := b.registry.List()

	if len(games) == 0 {
		respondWithMessage(s, i, "No games are currently supported.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**Supported Games:**\n\n")

	for _, g := range games {
		sb.WriteString(fmt.Sprintf("**%s** (`%s`)\n", g.Name, g.Type))
		sb.WriteString(fmt.Sprintf("  %s\n\n", g.Description))
	}

	sb.WriteString("Use `/register game:<game> player_id:<id>` to start tracking!")

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
