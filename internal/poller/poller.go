package poller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/flor3z/discord-bot/internal/riot"
	"github.com/flor3z/discord-bot/internal/storage"
)

// Poller periodically checks for new matches
type Poller struct {
	repo       *storage.Repository
	riotClient *riot.Client
	discord    *discordgo.Session
	interval   time.Duration

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// New creates a new Poller
func New(repo *storage.Repository, riotClient *riot.Client, discord *discordgo.Session, intervalSeconds int) *Poller {
	return &Poller{
		repo:       repo,
		riotClient: riotClient,
		discord:    discord,
		interval:   time.Duration(intervalSeconds) * time.Second,
		stopChan:   make(chan struct{}),
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
	// Get latest match IDs
	matchIDs, err := p.riotClient.GetMatchIDsByPUUID(ctx, summoner.PUUID, 1)
	if err != nil {
		slog.Error("Failed to get match IDs", "summoner", summoner.RiotID, "error", err)
		return
	}

	if len(matchIDs) == 0 {
		return
	}

	latestMatchID := matchIDs[0]

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
	match, err := p.riotClient.GetMatch(ctx, latestMatchID)
	if err != nil {
		slog.Error("Failed to get match details", "matchID", latestMatchID, "error", err)
		return
	}

	// Find the summoner in the match
	participant := match.FindParticipant(summoner.PUUID)
	if participant == nil {
		slog.Warn("Summoner not found in match", "summoner", summoner.RiotID, "matchID", latestMatchID)
		return
	}

	// Send notifications to all subscribed guilds
	p.sendNotifications(summoner, match, participant)

	// Update last match ID
	if err := p.repo.UpdateSummonerLastMatch(summoner.ID, latestMatchID); err != nil {
		slog.Error("Failed to update last match ID", "error", err)
	}
}

// sendNotifications sends match notifications to all subscribed guilds
func (p *Poller) sendNotifications(summoner *storage.Summoner, match *riot.Match, participant *riot.Participant) {
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

		embed := createMatchEmbed(summoner, match, participant)
		_, err = p.discord.ChannelMessageSendEmbed(settings.NotificationChannelID, embed)
		if err != nil {
			slog.Error("Failed to send notification", "guildID", sub.GuildID, "error", err)
		} else {
			slog.Info("Sent match notification", "summoner", summoner.RiotID, "guildID", sub.GuildID)
		}
	}
}

// createMatchEmbed creates a Discord embed for match notification
func createMatchEmbed(summoner *storage.Summoner, match *riot.Match, p *riot.Participant) *discordgo.MessageEmbed {
	// Determine color based on win/loss
	color := 0xE74C3C // Red for loss
	resultText := "Defeat"
	resultEmoji := ""
	if p.Win {
		color = 0x2ECC71 // Green for win
		resultText = "Victory"
		resultEmoji = ""
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
		Title: fmt.Sprintf("%s %s", resultEmoji, resultText),
		Color: color,
		Author: &discordgo.MessageEmbedAuthor{
			Name: summoner.RiotID,
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
