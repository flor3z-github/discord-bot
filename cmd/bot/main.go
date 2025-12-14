package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/flor3z/discord-bot/internal/bot"
	"github.com/flor3z/discord-bot/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set up logging
	setupLogging(cfg.LogLevel)

	slog.Info("Starting LoL Match Tracker Bot")

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and start the bot
	b, err := bot.New(cfg)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	// Start the bot
	if err := b.Start(ctx); err != nil {
		slog.Error("Failed to start bot", "error", err)
		os.Exit(1)
	}

	slog.Info("Bot is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down...")
	cancel()

	// Stop the bot gracefully
	if err := b.Stop(); err != nil {
		slog.Error("Error during shutdown", "error", err)
	}

	slog.Info("Bot stopped")
}

func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))
}
