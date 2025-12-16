package nexon

import (
	"context"
	"fmt"
	"net/url"
)

// CharacterOCID represents the response from /maplestory/v1/id
type CharacterOCID struct {
	OCID string `json:"ocid"`
}

// CharacterBasic represents the response from /maplestory/v1/character/basic
type CharacterBasic struct {
	CharacterName    string `json:"character_name"`
	CharacterLevel   int64  `json:"character_level"`
	CharacterExp     int64  `json:"character_exp"`
	CharacterExpRate string `json:"character_exp_rate"`
}

// GetCharacterOCID fetches the OCID for a character by name
func (c *Client) GetCharacterOCID(ctx context.Context, characterName string) (*CharacterOCID, error) {
	endpoint := fmt.Sprintf("%s/maplestory/v1/id?character_name=%s", BaseURL, url.QueryEscape(characterName))

	var result CharacterOCID
	if err := c.get(endpoint, &result); err != nil {
		return nil, err
	}

	if result.OCID == "" {
		return nil, fmt.Errorf("캐릭터를 찾을 수 없습니다: %s", characterName)
	}

	return &result, nil
}

// GetCharacterBasic fetches basic character information by OCID
func (c *Client) GetCharacterBasic(ctx context.Context, ocid string) (*CharacterBasic, error) {
	endpoint := fmt.Sprintf("%s/maplestory/v1/character/basic?ocid=%s", BaseURL, url.QueryEscape(ocid))

	var result CharacterBasic
	if err := c.get(endpoint, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
