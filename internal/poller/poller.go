package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/game"
	"github.com/flor3z/discord-bot/internal/games/lol"
	"github.com/flor3z/discord-bot/internal/storage"
)

// Poller periodically checks for new matches across all registered games
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
	slog.Info("Starting match poller", "interval", p.interval)

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

// poll checks all summoners for new matches
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

// checkSummoner checks a single summoner for new matches
func (p *Poller) checkSummoner(ctx context.Context, summoner *storage.Summoner) {
	// Get the appropriate tracker for this game
	tracker, err := p.registry.Get(game.GameType(summoner.GameType))
	if err != nil {
		slog.Error("Unknown game type for summoner", "summoner", summoner.RiotID, "gameType", summoner.GameType)
		return
	}

	// Get latest match ID
	latestMatchID, err := tracker.GetLatestMatchID(ctx, summoner.PUUID)
	if err != nil {
		slog.Error("Failed to get match IDs", "summoner", summoner.RiotID, "error", err)
		return
	}

	if latestMatchID == "" {
		return
	}

	// Check if this is a new match
	if latestMatchID == summoner.LastMatchID {
		slog.Debug("No new matches", "summoner", summoner.RiotID)
		return
	}

	// Skip if this is the first poll (no previous match recorded)
	if summoner.LastMatchID == "" {
		slog.Info("Setting initial match ID", "summoner", summoner.RiotID, "matchID", latestMatchID)
		p.repo.UpdateSummonerLastMatch(summoner.ID, latestMatchID)
		return
	}

	slog.Info("New match detected", "summoner", summoner.RiotID, "matchID", latestMatchID)

	// Fetch match details
	matchInfo, err := tracker.GetMatchDetails(ctx, latestMatchID)
	if err != nil {
		slog.Error("Failed to get match details", "matchID", latestMatchID, "error", err)
		return
	}

	// Send notifications to all subscribed guilds
	p.sendNotifications(summoner, tracker, matchInfo)

	// Update last match ID
	if err := p.repo.UpdateSummonerLastMatch(summoner.ID, latestMatchID); err != nil {
		slog.Error("Failed to update last match ID", "error", err)
	}
}

// sendNotifications sends match notifications to all subscribed guilds
func (p *Poller) sendNotifications(summoner *storage.Summoner, tracker game.Tracker, matchInfo *game.MatchInfo) {
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

		// Use game-specific formatter if available, otherwise use generic
		var embed *discordgo.MessageEmbed
		if lolTracker, ok := tracker.(*lol.Tracker); ok {
			// Use LoL-specific method that can find participant by PUUID
			embed = lolTracker.FormatNotificationByPUUID(summoner.RiotID, matchInfo, summoner.PUUID)
		} else {
			embed = tracker.FormatNotification(summoner.RiotID, matchInfo)
		}

		_, err = p.discord.ChannelMessageSendEmbed(settings.NotificationChannelID, embed)
		if err != nil {
			slog.Error("Failed to send notification", "guildID", sub.GuildID, "error", err)
		} else {
			slog.Info("Sent match notification", "summoner", summoner.RiotID, "guildID", sub.GuildID)
		}
	}
}
