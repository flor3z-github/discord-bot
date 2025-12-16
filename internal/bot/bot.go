package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/config"
	"github.com/flor3z/discord-bot/internal/game"
	"github.com/flor3z/discord-bot/internal/games/lol"
	"github.com/flor3z/discord-bot/internal/poller"
	"github.com/flor3z/discord-bot/internal/storage"
)

// Bot represents the Discord bot instance
type Bot struct {
	config   *config.Config
	session  *discordgo.Session
	repo     *storage.Repository
	registry *game.Registry
	poller   *poller.Poller
	commands []*discordgo.ApplicationCommand
}

// New creates a new Bot instance
func New(cfg *config.Config) (*Bot, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	// Initialize storage
	repo, err := storage.NewRepository(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize game registry and register trackers
	registry := game.NewRegistry()

	// Register League of Legends tracker
	lolTracker := lol.NewTracker(cfg.RiotAPIKey)
	registry.Register(lolTracker)

	// Future games can be registered here:
	// registry.Register(valorant.NewTracker(cfg.RiotAPIKey))
	// registry.Register(tft.NewTracker(cfg.RiotAPIKey))

	b := &Bot{
		config:   cfg,
		session:  session,
		repo:     repo,
		registry: registry,
	}

	// Register command handlers
	b.registerHandlers()

	return b, nil
}

// Start opens the Discord connection and starts background tasks
func (b *Bot) Start(ctx context.Context) error {
	// Open Discord connection
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}

	slog.Info("Connected to Discord", "user", b.session.State.User.Username)

	// Register slash commands
	if err := b.registerCommands(); err != nil {
		return fmt.Errorf("failed to register commands: %w", err)
	}

	// Start the match poller
	b.poller = poller.New(b.repo, b.registry, b.session, b.config.PollingIntervalSeconds)
	go b.poller.Start(ctx)

	return nil
}

// Stop gracefully shuts down the bot
func (b *Bot) Stop() error {
	// Stop the poller
	if b.poller != nil {
		b.poller.Stop()
	}

	// Remove registered commands (optional - comment out to keep commands)
	// b.removeCommands()

	// Close storage
	if b.repo != nil {
		b.repo.Close()
	}

	// Close Discord session
	if b.session != nil {
		return b.session.Close()
	}

	return nil
}

// registerHandlers sets up Discord event handlers
func (b *Bot) registerHandlers() {
	b.session.AddHandler(b.handleInteraction)
	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.Info("Bot is ready", "guilds", len(r.Guilds))
	})
}

// handleInteraction processes slash command interactions
func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	slog.Debug("Received command", "command", data.Name, "guild", i.GuildID)

	switch data.Name {
	case "register":
		b.handleRegister(s, i)
	case "unregister":
		b.handleUnregister(s, i)
	case "list":
		b.handleList(s, i)
	case "setchannel":
		b.handleSetChannel(s, i)
	case "games":
		b.handleGames(s, i)
	default:
		slog.Warn("Unknown command", "command", data.Name)
	}
}
