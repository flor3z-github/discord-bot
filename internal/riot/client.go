package riot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// Regional routing values for Asia (Korea)
	RegionalBaseURL = "https://asia.api.riotgames.com"
)

// Client is a Riot Games API client with rate limiting
type Client struct {
	apiKey     string
	httpClient *http.Client

	// Simple rate limiter
	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

// NewClient creates a new Riot API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		// Rate limit: ~20 requests per second (50ms between requests)
		minInterval: 50 * time.Millisecond,
	}
}

// doRequest performs an HTTP request with rate limiting
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	// Simple rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < c.minInterval {
		time.Sleep(c.minInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	// Add API key header
	req.Header.Set("X-Riot-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle rate limiting (429)
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		// Wait and retry once
		time.Sleep(1 * time.Second)
		return c.httpClient.Do(req)
	}

	return resp, nil
}

// get performs a GET request and decodes the JSON response
func (c *Client) get(url string, result interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
