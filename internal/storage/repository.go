package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Repository handles all database operations
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository with SQLite
func NewRepository(dbPath string) (*Repository, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	repo := &Repository{db: db}

	// Run migrations
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return repo, nil
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}

// migrate creates the database schema
func (r *Repository) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS summoners (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			puuid VARCHAR(100) UNIQUE NOT NULL,
			riot_id VARCHAR(50) NOT NULL,
			region VARCHAR(10) NOT NULL DEFAULT 'KR',
			last_match_id VARCHAR(50),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS guild_settings (
			guild_id VARCHAR(20) PRIMARY KEY,
			notification_channel_id VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS summoner_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			summoner_id INTEGER NOT NULL,
			guild_id VARCHAR(20) NOT NULL,
			registered_by VARCHAR(20) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (summoner_id) REFERENCES summoners(id) ON DELETE CASCADE,
			UNIQUE(summoner_id, guild_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_summoners_puuid ON summoners(puuid)`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_guild ON summoner_subscriptions(guild_id)`,
	}

	for _, migration := range migrations {
		if _, err := r.db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// Summoner operations

// CreateSummoner inserts a new summoner
func (r *Repository) CreateSummoner(s *Summoner) error {
	result, err := r.db.Exec(
		`INSERT INTO summoners (puuid, riot_id, region, last_match_id) VALUES (?, ?, ?, ?)`,
		s.PUUID, s.RiotID, s.Region, s.LastMatchID,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

// GetSummonerByPUUID finds a summoner by PUUID
func (r *Repository) GetSummonerByPUUID(puuid string) (*Summoner, error) {
	s := &Summoner{}
	err := r.db.QueryRow(
		`SELECT id, puuid, riot_id, region, last_match_id, created_at, updated_at FROM summoners WHERE puuid = ?`,
		puuid,
	).Scan(&s.ID, &s.PUUID, &s.RiotID, &s.Region, &s.LastMatchID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetSummonerByRiotID finds a summoner by Riot ID
func (r *Repository) GetSummonerByRiotID(riotID string) (*Summoner, error) {
	s := &Summoner{}
	err := r.db.QueryRow(
		`SELECT id, puuid, riot_id, region, last_match_id, created_at, updated_at FROM summoners WHERE riot_id = ?`,
		riotID,
	).Scan(&s.ID, &s.PUUID, &s.RiotID, &s.Region, &s.LastMatchID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateSummonerLastMatch updates the last match ID for a summoner
func (r *Repository) UpdateSummonerLastMatch(summonerID int64, matchID string) error {
	_, err := r.db.Exec(
		`UPDATE summoners SET last_match_id = ?, updated_at = ? WHERE id = ?`,
		matchID, time.Now(), summonerID,
	)
	return err
}

// GetAllSummoners returns all summoners with their subscription info
func (r *Repository) GetAllSummoners() ([]*Summoner, error) {
	rows, err := r.db.Query(
		`SELECT id, puuid, riot_id, region, last_match_id, created_at, updated_at FROM summoners`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summoners []*Summoner
	for rows.Next() {
		s := &Summoner{}
		if err := rows.Scan(&s.ID, &s.PUUID, &s.RiotID, &s.Region, &s.LastMatchID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		summoners = append(summoners, s)
	}

	return summoners, rows.Err()
}

// GetSummonersByGuild returns all summoners registered in a guild
func (r *Repository) GetSummonersByGuild(guildID string) ([]*Summoner, error) {
	rows, err := r.db.Query(
		`SELECT s.id, s.puuid, s.riot_id, s.region, s.last_match_id, s.created_at, s.updated_at
		 FROM summoners s
		 JOIN summoner_subscriptions sub ON s.id = sub.summoner_id
		 WHERE sub.guild_id = ?`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summoners []*Summoner
	for rows.Next() {
		s := &Summoner{}
		if err := rows.Scan(&s.ID, &s.PUUID, &s.RiotID, &s.Region, &s.LastMatchID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		summoners = append(summoners, s)
	}

	return summoners, rows.Err()
}

// Subscription operations

// CreateSubscription creates a new subscription
func (r *Repository) CreateSubscription(sub *Subscription) error {
	result, err := r.db.Exec(
		`INSERT INTO summoner_subscriptions (summoner_id, guild_id, registered_by) VALUES (?, ?, ?)`,
		sub.SummonerID, sub.GuildID, sub.RegisteredBy,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	sub.ID = id
	return nil
}

// DeleteSubscription removes a subscription
func (r *Repository) DeleteSubscription(summonerID int64, guildID string) error {
	_, err := r.db.Exec(
		`DELETE FROM summoner_subscriptions WHERE summoner_id = ? AND guild_id = ?`,
		summonerID, guildID,
	)
	return err
}

// GetSubscriptionsByGuild returns all subscriptions for a guild
func (r *Repository) GetSubscriptionsByGuild(guildID string) ([]*Subscription, error) {
	rows, err := r.db.Query(
		`SELECT id, summoner_id, guild_id, registered_by, created_at FROM summoner_subscriptions WHERE guild_id = ?`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.ID, &sub.SummonerID, &sub.GuildID, &sub.RegisteredBy, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}

	return subs, rows.Err()
}

// GetSubscriptionsBySummoner returns all guild subscriptions for a summoner
func (r *Repository) GetSubscriptionsBySummoner(summonerID int64) ([]*Subscription, error) {
	rows, err := r.db.Query(
		`SELECT id, summoner_id, guild_id, registered_by, created_at FROM summoner_subscriptions WHERE summoner_id = ?`,
		summonerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.ID, &sub.SummonerID, &sub.GuildID, &sub.RegisteredBy, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}

	return subs, rows.Err()
}

// Guild settings operations

// UpsertGuildSettings creates or updates guild settings
func (r *Repository) UpsertGuildSettings(settings *GuildSettings) error {
	_, err := r.db.Exec(
		`INSERT INTO guild_settings (guild_id, notification_channel_id) VALUES (?, ?)
		 ON CONFLICT(guild_id) DO UPDATE SET notification_channel_id = excluded.notification_channel_id`,
		settings.GuildID, settings.NotificationChannelID,
	)
	return err
}

// GetGuildSettings retrieves guild settings
func (r *Repository) GetGuildSettings(guildID string) (*GuildSettings, error) {
	settings := &GuildSettings{}
	err := r.db.QueryRow(
		`SELECT guild_id, notification_channel_id, created_at FROM guild_settings WHERE guild_id = ?`,
		guildID,
	).Scan(&settings.GuildID, &settings.NotificationChannelID, &settings.CreatedAt)
	if err != nil {
		return nil, err
	}
	return settings, nil
}
