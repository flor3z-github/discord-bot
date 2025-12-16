# Game Tracker Discord Bot

A Discord bot that monitors player activity across multiple games and sends real-time notifications.

## Supported Games

- **League of Legends** - Track summoner match history with detailed stats (KDA, CS, damage, vision)
- **MapleStory** - Track character level and experience progress

## Features

- **Multi-Game Support** - Register players from different games
- **Real-time Notifications** - Automatic alerts when tracked players have updates
- **Rich Embeds** - Color-coded results with detailed game-specific stats
- **Multi-server Support** - Works across multiple Discord servers with per-server settings

## Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/등록 <게임> <플레이어>` | Register a player for tracking | `/등록 lol Faker#KR1` |
| `/해제 <게임> <플레이어>` | Stop tracking a player | `/해제 lol Faker#KR1` |
| `/목록` | Show all tracked players | `/목록` |
| `/채널설정 <채널>` | Set notification channel | `/채널설정 #game-updates` |
| `/게임목록` | Show supported games | `/게임목록` |
| `/최근 <게임> <플레이어>` | Show recent player status | `/최근 maplestory 캐릭터명` |

## Requirements

- Go 1.21+
- Discord Bot Token ([Discord Developer Portal](https://discord.com/developers/applications))
- Riot API Key ([Riot Developer Portal](https://developer.riotgames.com/)) - for LoL
- Nexon API Key ([Nexon Open API](https://openapi.nexon.com/)) - for MapleStory

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
NEXON_API_KEY=your_nexon_api_key
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
| `RIOT_API_KEY` | Riot Games API key (for LoL) | - |
| `NEXON_API_KEY` | Nexon API key (for MapleStory) | - |
| `DATABASE_PATH` | SQLite database file path | `./data/bot.db` |
| `POLLING_INTERVAL_SECONDS` | Status check interval | `90` |
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
│   ├── game/
│   │   ├── tracker.go       # Game tracker interface
│   │   └── registry.go      # Game registry
│   ├── games/
│   │   ├── lol/             # League of Legends tracker
│   │   └── maplestory/      # MapleStory tracker
│   ├── riot/
│   │   ├── client.go        # Riot API client
│   │   ├── account.go       # Account-V1 API
│   │   └── match.go         # Match-V5 API
│   ├── nexon/
│   │   ├── client.go        # Nexon API client
│   │   └── maplestory.go    # MapleStory API
│   ├── storage/
│   │   ├── models.go        # Data models
│   │   └── repository.go    # SQLite operations
│   └── poller/
│       └── poller.go        # Background polling
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

## License

Apache 2.0
