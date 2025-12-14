package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/storage"
)

// Slash command definitions
var commandDefinitions = []*discordgo.ApplicationCommand{
	{
		Name:        "register",
		Description: "Register a summoner to track match history",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "riot_id",
				Description: "Riot ID in format: GameName#TagLine (e.g., Faker#KR1)",
				Required:    true,
			},
		},
	},
	{
		Name:        "unregister",
		Description: "Stop tracking a summoner's match history",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "riot_id",
				Description: "Riot ID in format: GameName#TagLine (e.g., Faker#KR1)",
				Required:    true,
			},
		},
	},
	{
		Name:        "list",
		Description: "List all registered summoners in this server",
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
}

// registerCommands registers all slash commands with Discord
func (b *Bot) registerCommands() error {
	slog.Info("Registering slash commands")

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
	riotID := i.ApplicationCommandData().Options[0].StringValue()

	// Respond immediately to avoid timeout
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Parse Riot ID
	gameName, tagLine, err := parseRiotID(riotID)
	if err != nil {
		b.editResponse(s, i, fmt.Sprintf("Invalid Riot ID format. Use: `GameName#TagLine` (e.g., `Faker#KR1`)"))
		return
	}

	// Look up PUUID from Riot API
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account, err := b.riotClient.GetAccountByRiotID(ctx, gameName, tagLine)
	if err != nil {
		slog.Error("Failed to look up Riot account", "riotID", riotID, "error", err)
		b.editResponse(s, i, fmt.Sprintf("Could not find summoner `%s`. Please check the Riot ID and try again.", riotID))
		return
	}

	// Get initial match ID
	matchIDs, err := b.riotClient.GetMatchIDsByPUUID(ctx, account.PUUID, 1)
	if err != nil {
		slog.Warn("Failed to get initial match history", "puuid", account.PUUID, "error", err)
		// Continue without last match ID - will be set on first poll
	}

	lastMatchID := ""
	if len(matchIDs) > 0 {
		lastMatchID = matchIDs[0]
	}

	// Store summoner
	summoner := &storage.Summoner{
		PUUID:       account.PUUID,
		RiotID:      fmt.Sprintf("%s#%s", account.GameName, account.TagLine),
		Region:      "KR", // Default to KR for now
		LastMatchID: lastMatchID,
	}

	if err := b.repo.CreateSummoner(summoner); err != nil {
		// Check if already exists
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			b.editResponse(s, i, fmt.Sprintf("Summoner `%s` is already registered.", summoner.RiotID))
			return
		}
		slog.Error("Failed to save summoner", "error", err)
		b.editResponse(s, i, "Failed to register summoner. Please try again.")
		return
	}

	// Create subscription for this guild
	sub := &storage.Subscription{
		SummonerID:   summoner.ID,
		GuildID:      i.GuildID,
		RegisteredBy: i.Member.User.ID,
	}

	if err := b.repo.CreateSubscription(sub); err != nil {
		slog.Error("Failed to create subscription", "error", err)
		b.editResponse(s, i, "Summoner saved but failed to create subscription.")
		return
	}

	b.editResponse(s, i, fmt.Sprintf("Successfully registered `%s` for match tracking!", summoner.RiotID))
}

// handleUnregister handles the /unregister command
func (b *Bot) handleUnregister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	riotID := i.ApplicationCommandData().Options[0].StringValue()

	// Parse and normalize Riot ID
	gameName, tagLine, err := parseRiotID(riotID)
	if err != nil {
		respondWithMessage(s, i, "Invalid Riot ID format. Use: `GameName#TagLine`")
		return
	}
	normalizedRiotID := fmt.Sprintf("%s#%s", gameName, tagLine)

	// Find summoner
	summoner, err := b.repo.GetSummonerByRiotID(normalizedRiotID)
	if err != nil {
		respondWithMessage(s, i, fmt.Sprintf("Summoner `%s` is not registered.", normalizedRiotID))
		return
	}

	// Delete subscription for this guild
	if err := b.repo.DeleteSubscription(summoner.ID, i.GuildID); err != nil {
		slog.Error("Failed to delete subscription", "error", err)
		respondWithMessage(s, i, "Failed to unregister summoner. Please try again.")
		return
	}

	respondWithMessage(s, i, fmt.Sprintf("Successfully unregistered `%s`.", normalizedRiotID))
}

// handleList handles the /list command
func (b *Bot) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	summoners, err := b.repo.GetSummonersByGuild(i.GuildID)
	if err != nil {
		slog.Error("Failed to get summoners", "error", err)
		respondWithMessage(s, i, "Failed to retrieve summoner list.")
		return
	}

	if len(summoners) == 0 {
		respondWithMessage(s, i, "No summoners are registered in this server.\nUse `/register` to add one!")
		return
	}

	// Build list
	var sb strings.Builder
	sb.WriteString("**Registered Summoners:**\n")
	for idx, summoner := range summoners {
		sb.WriteString(fmt.Sprintf("%d. `%s`\n", idx+1, summoner.RiotID))
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

// Helper functions

func parseRiotID(riotID string) (gameName, tagLine string, err error) {
	parts := strings.Split(riotID, "#")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format: must contain exactly one '#'")
	}

	gameName = strings.TrimSpace(parts[0])
	tagLine = strings.TrimSpace(parts[1])

	if gameName == "" || tagLine == "" {
		return "", "", fmt.Errorf("game name and tag line cannot be empty")
	}

	return gameName, tagLine, nil
}

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
