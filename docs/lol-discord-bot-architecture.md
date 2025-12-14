# League of Legends Discord Match Tracker Bot

A Discord bot that monitors summoner match history and sends real-time notifications when new games are completed.

---

## Table of Contents

1. [Overview](#overview)
2. [System Architecture](#system-architecture)
3. [Core Components](#core-components)
4. [Data Flow](#data-flow)
5. [Riot API Integration](#riot-api-integration)
6. [Database Schema](#database-schema)
7. [Discord Bot Commands](#discord-bot-commands)
8. [Implementation Phases](#implementation-phases)
9. [Technical Considerations](#technical-considerations)
10. [Deployment](#deployment)

---

## Overview

### Purpose

This bot allows Discord users to register League of Legends summoners for monitoring. When a registered summoner completes a match, the bot automatically fetches the match details and posts a formatted summary to a designated Discord channel.

### Key Features

- Register/unregister summoners for tracking via Discord commands
- Automatic polling of match history at configurable intervals
- Real-time notifications with detailed match statistics
- Support for multiple summoners and Discord servers

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Discord Server                              │
│  ┌─────────────┐                              ┌─────────────────┐   │
│  │    User     │ ──── /register, /list ────▶  │  Discord Bot    │   │
│  └─────────────┘                              │    (Client)     │   │
│                                               └────────┬────────┘   │
└────────────────────────────────────────────────────────┼────────────┘
                                                         │
                    ┌────────────────────────────────────┼────────────┐
                    │              Bot Server            │            │
                    │                                    ▼            │
                    │  ┌─────────────────────────────────────────┐   │
                    │  │           Command Handler               │   │
                    │  │  - Process user commands                │   │
                    │  │  - Validate Riot ID format              │   │
                    │  └──────────────────┬──────────────────────┘   │
                    │                     │                           │
                    │                     ▼                           │
                    │  ┌─────────────────────────────────────────┐   │
                    │  │           Summoner Service              │   │
                    │  │  - Resolve Riot ID → PUUID              │   │
                    │  │  - Manage registered summoners          │   │
                    │  └──────────────────┬──────────────────────┘   │
                    │                     │                           │
                    │         ┌───────────┴───────────┐               │
                    │         ▼                       ▼               │
                    │  ┌─────────────┐      ┌─────────────────────┐   │
                    │  │  Database   │      │  Background Task    │   │
                    │  │  (SQLite/   │◀────▶│  (Match Poller)     │   │
                    │  │  PostgreSQL)│      │  - Runs every 1-2m  │   │
                    │  └─────────────┘      └──────────┬──────────┘   │
                    │                                  │               │
                    └──────────────────────────────────┼───────────────┘
                                                       │
                                                       ▼
                    ┌──────────────────────────────────────────────────┐
                    │                  Riot Games API                  │
                    │  ┌────────────┐ ┌────────────┐ ┌──────────────┐  │
                    │  │ Account-V1 │ │  Match-V5  │ │  League-V4   │  │
                    │  │ (PUUID)    │ │ (History)  │ │ (Rank Info)  │  │
                    │  └────────────┘ └────────────┘ └──────────────┘  │
                    └──────────────────────────────────────────────────┘
```

---

## Core Components

### 1. Discord Bot Client

Handles all Discord-related functionality including command processing and message sending.

**Responsibilities:**
- Listen for slash commands
- Send embedded match notifications
- Manage per-server configurations

### 2. Command Handler

Processes user commands and validates input.

**Supported Commands:**
- `/register <RiotID#Tag>` - Add summoner to watchlist
- `/unregister <RiotID#Tag>` - Remove summoner from watchlist
- `/list` - Show all registered summoners
- `/setchannel` - Set notification channel

### 3. Summoner Service

Manages summoner data and interacts with Riot API for account information.

**Functions:**
- Resolve Riot ID to PUUID
- Store and retrieve summoner data
- Handle nickname changes gracefully

### 4. Match Poller (Background Task)

Continuously monitors registered summoners for new matches.

**Workflow:**
1. Iterate through all registered summoners
2. Fetch recent match IDs from Riot API
3. Compare with last known match ID
4. Trigger notification if new match detected
5. Update last known match ID in database

### 5. Match Notification Service

Formats and sends match data to Discord.

**Features:**
- Rich embed messages with champion icons
- Color-coded results (green for win, red for loss)
- Key statistics display (KDA, CS, damage, vision)

---

## Data Flow

### Registration Flow

```
User sends /register Faker#KR1
           │
           ▼
┌─────────────────────────┐
│  Validate Riot ID       │
│  format (Name#Tag)      │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Call Account-V1 API    │
│  GET /riot/account/v1/  │
│  accounts/by-riot-id/   │
│  {gameName}/{tagLine}   │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Receive PUUID          │
│  (Permanent identifier) │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Fetch initial match    │
│  history (latest ID)    │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Store in database:     │
│  - PUUID                │
│  - Riot ID              │
│  - Last Match ID        │
│  - Discord Server ID    │
│  - Channel ID           │
└───────────┬─────────────┘
            │
            ▼
   Send confirmation message
```

### Match Detection Flow

```
Background Task (every 60-120 seconds)
           │
           ▼
┌─────────────────────────┐
│  Load all registered    │
│  summoners from DB      │
└───────────┬─────────────┘
            │
            ▼
   For each summoner:
            │
            ▼
┌─────────────────────────┐
│  Call Match-V5 API      │
│  GET /lol/match/v5/     │
│  matches/by-puuid/      │
│  {puuid}/ids?count=1    │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Compare latest match   │
│  ID with stored ID      │
└───────────┬─────────────┘
            │
       ┌────┴────┐
       │         │
    Same      Different
       │         │
       ▼         ▼
    Skip    ┌─────────────────────────┐
            │  Fetch match details    │
            │  GET /lol/match/v5/     │
            │  matches/{matchId}      │
            └───────────┬─────────────┘
                        │
                        ▼
            ┌─────────────────────────┐
            │  Extract player stats:  │
            │  - Champion played      │
            │  - K/D/A                 │
            │  - CS & Gold            │
            │  - Damage dealt         │
            │  - Win/Loss             │
            └───────────┬─────────────┘
                        │
                        ▼
            ┌─────────────────────────┐
            │  Format Discord Embed   │
            │  & send to channel      │
            └───────────┬─────────────┘
                        │
                        ▼
            ┌─────────────────────────┐
            │  Update last match ID   │
            │  in database            │
            └─────────────────────────┘
```

---

## Riot API Integration

### Required Endpoints

| Endpoint | Purpose | Rate Limit Impact |
|----------|---------|-------------------|
| `Account-V1` | Resolve Riot ID → PUUID | Low (registration only) |
| `Match-V5 /ids` | Get recent match IDs | High (polling) |
| `Match-V5 /matches/{id}` | Get match details | Medium (new matches only) |
| `League-V4` (optional) | Get rank information | Low |

### API Call Examples

**1. Get PUUID from Riot ID**
```
GET https://asia.api.riotgames.com/riot/account/v1/accounts/by-riot-id/{gameName}/{tagLine}
Headers: X-Riot-Token: {API_KEY}

Response:
{
  "puuid": "abc123...",
  "gameName": "Faker",
  "tagLine": "KR1"
}
```

**2. Get Recent Match IDs**
```
GET https://asia.api.riotgames.com/lol/match/v5/matches/by-puuid/{puuid}/ids?count=5
Headers: X-Riot-Token: {API_KEY}

Response:
["KR_1234567890", "KR_1234567889", ...]
```

**3. Get Match Details**
```
GET https://asia.api.riotgames.com/lol/match/v5/matches/{matchId}
Headers: X-Riot-Token: {API_KEY}

Response:
{
  "info": {
    "gameDuration": 1823,
    "participants": [...],
    ...
  }
}
```

### Regional Routing

| Region | Account API | Match API |
|--------|-------------|-----------|
| Korea | asia.api.riotgames.com | asia.api.riotgames.com |
| NA | americas.api.riotgames.com | americas.api.riotgames.com |
| EU | europe.api.riotgames.com | europe.api.riotgames.com |

---

## Database Schema

### Tables

**summoners**
| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| puuid | VARCHAR(100) | Riot PUUID (unique) |
| riot_id | VARCHAR(50) | Display name (Name#Tag) |
| region | VARCHAR(10) | Player region (KR, NA, etc.) |
| last_match_id | VARCHAR(50) | Most recent processed match |
| created_at | TIMESTAMP | Registration time |
| updated_at | TIMESTAMP | Last update time |

**guild_settings**
| Column | Type | Description |
|--------|------|-------------|
| guild_id | BIGINT | Discord server ID (PK) |
| notification_channel_id | BIGINT | Channel for notifications |
| created_at | TIMESTAMP | Setup time |

**summoner_subscriptions**
| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| summoner_id | INTEGER | FK to summoners |
| guild_id | BIGINT | FK to guild_settings |
| registered_by | BIGINT | Discord user ID |
| created_at | TIMESTAMP | Subscription time |

### Entity Relationship

```
┌─────────────────┐       ┌──────────────────────┐       ┌─────────────────┐
│   summoners     │       │ summoner_subscriptions│       │ guild_settings  │
├─────────────────┤       ├──────────────────────┤       ├─────────────────┤
│ id (PK)         │◀──────│ summoner_id (FK)     │       │ guild_id (PK)   │
│ puuid           │       │ guild_id (FK)        │──────▶│ notification_   │
│ riot_id         │       │ registered_by        │       │   channel_id    │
│ region          │       │ created_at           │       │ created_at      │
│ last_match_id   │       └──────────────────────┘       └─────────────────┘
│ created_at      │
│ updated_at      │
└─────────────────┘
```

---

## Discord Bot Commands

### User Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/register <riot_id>` | Register a summoner for tracking | `/register Hide on bush#KR1` |
| `/unregister <riot_id>` | Remove a summoner from tracking | `/unregister Hide on bush#KR1` |
| `/list` | List all registered summoners | `/list` |
| `/recent <riot_id>` | Show recent match history | `/recent Faker#KR1` |

### Admin Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/setchannel` | Set notification channel | `/setchannel #lol-updates` |
| `/config` | View/edit bot configuration | `/config` |

### Sample Notification Embed

```
┌────────────────────────────────────────────────┐
│  🏆 Victory                                    │
│  ─────────────────────────────────────────     │
│  👤 Faker#KR1                                  │
│  🎮 Ranked Solo/Duo                            │
│                                                │
│  ┌──────┐  Ahri                                │
│  │ ICON │  12 / 3 / 8  (KDA: 6.67)             │
│  └──────┘                                      │
│                                                │
│  📊 Stats                                      │
│  ├─ CS: 287 (8.2/min)                          │
│  ├─ Damage: 32,451                             │
│  ├─ Gold: 14,230                               │
│  └─ Vision: 24                                 │
│                                                │
│  ⏱️ Duration: 35:12                            │
│  📅 2024-01-15 14:32 KST                       │
└────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1)

- [ ] Set up project structure
- [ ] Configure Discord.py / Discord.js bot
- [ ] Implement basic slash commands
- [ ] Create database models
- [ ] Set up Riot API client with rate limiting

### Phase 2: Core Features (Week 2)

- [ ] Implement summoner registration flow
- [ ] Build match polling background task
- [ ] Create match detection logic
- [ ] Design and implement Discord embeds

### Phase 3: Polish & Testing (Week 3)

- [ ] Add error handling and logging
- [ ] Implement rate limit management
- [ ] Add support for multiple regions
- [ ] Write unit and integration tests
- [ ] Create configuration management

### Phase 4: Deployment (Week 4)

- [ ] Set up production database
- [ ] Deploy to cloud server
- [ ] Configure monitoring and alerts
- [ ] Document setup process
- [ ] Apply for Riot production API key

---

## Technical Considerations

### Rate Limiting Strategy

**Riot API Limits (Development Key):**
- 20 requests per second
- 100 requests per 2 minutes

**Mitigation Strategies:**

1. **Request Queuing**: Implement a queue with configurable delay between requests
2. **Adaptive Polling**: Increase polling interval as summoner count grows
3. **Caching**: Cache summoner data to reduce redundant API calls
4. **Batch Processing**: Group API calls efficiently

**Recommended Polling Formula:**
```
polling_interval = max(60, summoner_count * 1.2) seconds
```

### Error Handling

| Error Type | Handling Strategy |
|------------|-------------------|
| 404 - Summoner not found | Notify user, suggest checking spelling |
| 429 - Rate limited | Back off exponentially, retry after delay |
| 503 - Service unavailable | Retry with exponential backoff |
| Network timeout | Retry up to 3 times |

### Security Best Practices

1. **API Key Protection**: Store in environment variables, never commit to git
2. **Input Validation**: Sanitize all user inputs
3. **Database Security**: Use parameterized queries to prevent SQL injection
4. **Permission Checks**: Verify user permissions for admin commands

---

## Deployment

### Recommended Stack

| Component | Recommendation |
|-----------|----------------|
| Language | Python 3.10+ or Node.js 18+ |
| Discord Library | discord.py / discord.js |
| Database | SQLite (small) / PostgreSQL (production) |
| Hosting | Oracle Cloud Free Tier / AWS EC2 / Railway |
| Process Manager | PM2 (Node) / systemd (Python) |

### Environment Variables

```env
# Discord
DISCORD_BOT_TOKEN=your_discord_bot_token
DISCORD_APPLICATION_ID=your_application_id

# Riot Games
RIOT_API_KEY=your_riot_api_key
RIOT_DEFAULT_REGION=asia

# Database
DATABASE_URL=sqlite:///bot.db

# Configuration
POLLING_INTERVAL_SECONDS=90
LOG_LEVEL=INFO
```

### Docker Deployment (Optional)

```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
CMD ["python", "main.py"]
```

---

## Next Steps

1. **Get API Keys**
   - Create Discord application at [Discord Developer Portal](https://discord.com/developers/applications)
   - Register at [Riot Developer Portal](https://developer.riotgames.com/)

2. **Choose Tech Stack**
   - Select programming language (Python recommended for beginners)
   - Set up development environment

3. **Start Implementation**
   - Begin with Phase 1 tasks
   - Test incrementally

---

## Resources

- [Riot Games API Documentation](https://developer.riotgames.com/docs/lol)
- [Discord.py Documentation](https://discordpy.readthedocs.io/)
- [Discord.js Documentation](https://discord.js.org/)
- [Data Dragon (Champion/Item Assets)](https://developer.riotgames.com/docs/lol#data-dragon)

---

*Document Version: 1.0*  
*Last Updated: December 2024*
