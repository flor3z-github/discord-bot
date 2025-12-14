# LoL Match Tracker Discord Bot

A Discord bot that monitors League of Legends summoner match history and sends real-time notifications when new games are completed.

## Features

- **Summoner Tracking** - Register summoners to monitor their match history
- **Real-time Notifications** - Automatic alerts when tracked summoners complete a match
- **Rich Embeds** - Color-coded match results with detailed stats (KDA, CS, damage, vision)
- **Multi-server Support** - Works across multiple Discord servers with per-server settings
- **Rate Limited** - Built-in Riot API rate limiting to prevent throttling

## Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/register <riot_id>` | Track a summoner's matches | `/register Faker#KR1` |
| `/unregister <riot_id>` | Stop tracking a summoner | `/unregister Faker#KR1` |
| `/list` | Show all tracked summoners | `/list` |
| `/setchannel <channel>` | Set notification channel | `/setchannel #lol-updates` |

## Requirements

- Go 1.21+
- Discord Bot Token ([Discord Developer Portal](https://discord.com/developers/applications))
- Riot API Key ([Riot Developer Portal](https://developer.riotgames.com/))

## Quick Start

### 1. Clone and build

```bash
git clone https://github.com/flor3z/discord-bot.git
cd discord-bot
go mod tidy
go build ./cmd/bot
```

### 2. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your credentials:

```env
DISCORD_BOT_TOKEN=your_discord_bot_token
RIOT_API_KEY=your_riot_api_key
```

### 3. Run the bot

```bash
go run ./cmd/bot
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DISCORD_BOT_TOKEN` | Discord bot token (required) | - |
| `DISCORD_APPLICATION_ID` | Discord application ID | - |
| `RIOT_API_KEY` | Riot Games API key (required) | - |
| `DATABASE_PATH` | SQLite database file path | `./data/bot.db` |
| `POLLING_INTERVAL_SECONDS` | Match check interval | `90` |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | `info` |

## Project Structure

```
discord-bot/
├── cmd/bot/
│   └── main.go              # Application entry point
├── internal/
│   ├── bot/
│   │   ├── bot.go           # Discord client & lifecycle
│   │   └── commands.go      # Slash command handlers
│   ├── config/
│   │   └── config.go        # Environment configuration
│   ├── riot/
│   │   ├── client.go        # HTTP client with rate limiting
│   │   ├── account.go       # Account-V1 API
│   │   └── match.go         # Match-V5 API
│   ├── storage/
│   │   ├── models.go        # Data models
│   │   └── repository.go    # SQLite operations
│   └── poller/
│       └── poller.go        # Background match polling
├── .env.example             # Environment template
└── go.mod                   # Go module
```

## Discord Bot Setup

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a new application
3. Go to **Bot** section and create a bot
4. Copy the bot token to your `.env` file
5. Enable **SERVER MEMBERS INTENT** under Privileged Gateway Intents
6. Go to **OAuth2 > URL Generator**
7. Select scopes: `bot`, `applications.commands`
8. Select permissions: `Send Messages`, `Embed Links`, `Use Slash Commands`
9. Use the generated URL to invite the bot to your server

## Regional Support

Currently configured for **Korea (KR)** region using `asia.api.riotgames.com`.

To support other regions, modify the `RegionalBaseURL` in `internal/riot/client.go`:

| Region | Base URL |
|--------|----------|
| Korea, Japan, SEA | `asia.api.riotgames.com` |
| NA, BR, LAN, LAS | `americas.api.riotgames.com` |
| EU, TR, RU | `europe.api.riotgames.com` |

## License

Apache 2.0
