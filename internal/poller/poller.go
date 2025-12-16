package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/game"
	"github.com/flor3z/discord-bot/internal/storage"
)

// Poller periodically checks for state changes across all registered games
type Poller struct {
	repo     *storage.Repository
	registry *game.Registry
	discord  *discordgo.Session
	interval time.Duration

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// New creates a new Poller with the game registry
func New(repo *storage.Repository, registry *game.Registry, discord *discordgo.Session, intervalSeconds int) *Poller {
	return &Poller{
		repo:     repo,
		registry: registry,
		discord:  discord,
		interval: time.Duration(intervalSeconds) * time.Second,
		stopChan: make(chan struct{}),
	}
}

// Start begins the polling loop
func (p *Poller) Start(ctx context.Context) {
	slog.Info("Starting poller", "interval", p.interval)

	p.wg.Add(1)
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Initial poll
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Poller stopped (context cancelled)")
			return
		case <-p.stopChan:
			slog.Info("Poller stopped")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// Stop signals the poller to stop
func (p *Poller) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}

// poll checks all players for state changes
func (p *Poller) poll(ctx context.Context) {
	summoners, err := p.repo.GetAllSummoners()
	if err != nil {
		slog.Error("Failed to get summoners", "error", err)
		return
	}

	if len(summoners) == 0 {
		slog.Debug("No summoners to poll")
		return
	}

	slog.Debug("Polling summoners", "count", len(summoners))

	for _, summoner := range summoners {
		select {
		case <-ctx.Done():
			return
		default:
			p.checkSummoner(ctx, summoner)
		}
	}
}

// checkSummoner checks a single player for state changes
func (p *Poller) checkSummoner(ctx context.Context, summoner *storage.Summoner) {
	// Get the appropriate tracker for this game
	tracker, err := p.registry.Get(game.GameType(summoner.GameType))
	if err != nil {
		slog.Error("Unknown game type for summoner", "summoner", summoner.RiotID, "gameType", summoner.GameType)
		return
	}

	// Get current state
	currentState, err := tracker.GetCurrentState(ctx, summoner.PUUID)
	if err != nil {
		slog.Error("Failed to get current state", "summoner", summoner.RiotID, "error", err)
		return
	}

	if currentState == "" {
		return
	}

	// Check if state has changed
	if currentState == summoner.LastMatchID {
		slog.Debug("No state change", "summoner", summoner.RiotID)
		return
	}

	// Skip if this is the first poll (no previous state recorded)
	if summoner.LastMatchID == "" {
		slog.Info("Setting initial state", "summoner", summoner.RiotID, "state", currentState)
		p.repo.UpdateSummonerLastMatch(summoner.ID, currentState)
		return
	}

	slog.Info("State change detected", "summoner", summoner.RiotID, "newState", currentState)

	// Send notifications to all subscribed guilds
	p.sendNotifications(ctx, summoner, tracker, currentState)

	// Update stored state
	if err := p.repo.UpdateSummonerLastMatch(summoner.ID, currentState); err != nil {
		slog.Error("Failed to update state", "error", err)
	}
}

// sendNotifications sends notifications to all subscribed guilds
func (p *Poller) sendNotifications(ctx context.Context, summoner *storage.Summoner, tracker game.Tracker, stateID string) {
	subs, err := p.repo.GetSubscriptionsBySummoner(summoner.ID)
	if err != nil {
		slog.Error("Failed to get subscriptions", "error", err)
		return
	}

	for _, sub := range subs {
		settings, err := p.repo.GetGuildSettings(sub.GuildID)
		if err != nil {
			slog.Warn("No notification channel set for guild", "guildID", sub.GuildID)
			continue
		}

		// Create notification using the unified interface
		embed, err := tracker.CreateNotification(ctx, summoner.PUUID, summoner.RiotID, stateID)
		if err != nil {
			slog.Error("Failed to create notification", "summoner", summoner.RiotID, "error", err)
			continue
		}

		_, err = p.discord.ChannelMessageSendEmbed(settings.NotificationChannelID, embed)
		if err != nil {
			slog.Error("Failed to send notification", "guildID", sub.GuildID, "error", err)
		} else {
			slog.Info("Sent notification", "summoner", summoner.RiotID, "guildID", sub.GuildID)
		}
	}
}
