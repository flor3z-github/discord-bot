package riot

import (
	"context"
	"fmt"
)

// Match represents match data from the Match-V5 API
type Match struct {
	Metadata MatchMetadata `json:"metadata"`
	Info     MatchInfo     `json:"info"`
}

// MatchMetadata contains match metadata
type MatchMetadata struct {
	MatchID      string   `json:"matchId"`
	Participants []string `json:"participants"` // PUUIDs
}

// MatchInfo contains detailed match information
type MatchInfo struct {
	GameDuration  int64         `json:"gameDuration"` // in seconds
	GameMode      string        `json:"gameMode"`
	GameType      string        `json:"gameType"`
	QueueID       int           `json:"queueId"`
	GameCreation  int64         `json:"gameCreation"` // Unix timestamp in ms
	Participants  []Participant `json:"participants"`
	GameEndTimestamp int64      `json:"gameEndTimestamp"` // Unix timestamp in ms
}

// Participant represents a player in the match
type Participant struct {
	PUUID                     string `json:"puuid"`
	SummonerName              string `json:"summonerName"`
	RiotIdGameName            string `json:"riotIdGameName"`
	RiotIdTagline             string `json:"riotIdTagline"`
	ChampionName              string `json:"championName"`
	ChampionID                int    `json:"championId"`
	TeamID                    int    `json:"teamId"`
	Win                       bool   `json:"win"`
	Kills                     int    `json:"kills"`
	Deaths                    int    `json:"deaths"`
	Assists                   int    `json:"assists"`
	TotalMinionsKilled        int    `json:"totalMinionsKilled"`
	NeutralMinionsKilled      int    `json:"neutralMinionsKilled"`
	GoldEarned                int    `json:"goldEarned"`
	TotalDamageDealtToChampions int  `json:"totalDamageDealtToChampions"`
	VisionScore               int    `json:"visionScore"`
	WardsPlaced               int    `json:"wardsPlaced"`
	Item0                     int    `json:"item0"`
	Item1                     int    `json:"item1"`
	Item2                     int    `json:"item2"`
	Item3                     int    `json:"item3"`
	Item4                     int    `json:"item4"`
	Item5                     int    `json:"item5"`
	Item6                     int    `json:"item6"` // Trinket
	SummonerSpell1ID          int    `json:"summoner1Id"`
	SummonerSpell2ID          int    `json:"summoner2Id"`
}

// GetMatchIDsByPUUID retrieves recent match IDs for a player
func (c *Client) GetMatchIDsByPUUID(ctx context.Context, puuid string, count int) ([]string, error) {
	if count <= 0 {
		count = 5
	}
	if count > 100 {
		count = 100
	}

	endpoint := fmt.Sprintf("%s/lol/match/v5/matches/by-puuid/%s/ids?count=%d",
		RegionalBaseURL, puuid, count)

	var matchIDs []string
	if err := c.get(endpoint, &matchIDs); err != nil {
		return nil, fmt.Errorf("failed to get match IDs: %w", err)
	}

	return matchIDs, nil
}

// GetMatch retrieves detailed match information
func (c *Client) GetMatch(ctx context.Context, matchID string) (*Match, error) {
	endpoint := fmt.Sprintf("%s/lol/match/v5/matches/%s", RegionalBaseURL, matchID)

	var match Match
	if err := c.get(endpoint, &match); err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}

	return &match, nil
}

// FindParticipant finds a participant in the match by PUUID
func (m *Match) FindParticipant(puuid string) *Participant {
	for i := range m.Info.Participants {
		if m.Info.Participants[i].PUUID == puuid {
			return &m.Info.Participants[i]
		}
	}
	return nil
}

// GetQueueName returns a human-readable queue name
func GetQueueName(queueID int) string {
	queueNames := map[int]string{
		420: "Ranked Solo/Duo",
		440: "Ranked Flex",
		400: "Normal Draft",
		430: "Normal Blind",
		450: "ARAM",
		900: "URF",
		1020: "One for All",
		1300: "Nexus Blitz",
		1400: "Ultimate Spellbook",
		1700: "Arena",
	}

	if name, ok := queueNames[queueID]; ok {
		return name
	}
	return "Custom Game"
}
