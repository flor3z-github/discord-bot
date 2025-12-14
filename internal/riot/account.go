package riot

import (
	"context"
	"fmt"
	"net/url"
)

// Account represents a Riot account from the Account-V1 API
type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// GetAccountByRiotID retrieves account information by Riot ID
// Uses the Account-V1 API endpoint
func (c *Client) GetAccountByRiotID(ctx context.Context, gameName, tagLine string) (*Account, error) {
	// URL encode the parameters
	encodedGameName := url.PathEscape(gameName)
	encodedTagLine := url.PathEscape(tagLine)

	endpoint := fmt.Sprintf("%s/riot/account/v1/accounts/by-riot-id/%s/%s",
		RegionalBaseURL, encodedGameName, encodedTagLine)

	var account Account
	if err := c.get(endpoint, &account); err != nil {
		return nil, fmt.Errorf("failed to get account by Riot ID: %w", err)
	}

	return &account, nil
}

// GetAccountByPUUID retrieves account information by PUUID
func (c *Client) GetAccountByPUUID(ctx context.Context, puuid string) (*Account, error) {
	endpoint := fmt.Sprintf("%s/riot/account/v1/accounts/by-puuid/%s",
		RegionalBaseURL, puuid)

	var account Account
	if err := c.get(endpoint, &account); err != nil {
		return nil, fmt.Errorf("failed to get account by PUUID: %w", err)
	}

	return &account, nil
}
