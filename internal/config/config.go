package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration values for the bot
type Config struct {
	// Discord
	DiscordToken         string
	DiscordApplicationID string

	// Riot API
	RiotAPIKey string

	// Nexon API
	NexonAPIKey string

	// Database
	DatabasePath string

	// Polling
	PollingIntervalSeconds int

	// Logging
	LogLevel string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		DiscordToken:         os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordApplicationID: os.Getenv("DISCORD_APPLICATION_ID"),
		RiotAPIKey:           os.Getenv("RIOT_API_KEY"),
		NexonAPIKey:          os.Getenv("NEXON_API_KEY"),
		DatabasePath:         getEnvOrDefault("DATABASE_PATH", "./data/bot.db"),
		LogLevel:             getEnvOrDefault("LOG_LEVEL", "info"),
	}

	// Parse polling interval
	pollingStr := getEnvOrDefault("POLLING_INTERVAL_SECONDS", "90")
	polling, err := strconv.Atoi(pollingStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POLLING_INTERVAL_SECONDS: %w", err)
	}
	cfg.PollingIntervalSeconds = polling

	// Validate required fields
	if cfg.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN is required")
	}
	if cfg.RiotAPIKey == "" {
		return nil, fmt.Errorf("RIOT_API_KEY is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
