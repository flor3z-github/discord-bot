package nexon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	BaseURL = "https://open.api.nexon.com"
)

// APIError represents a Nexon API error response
type APIError struct {
	Error struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

// Client is a Nexon Open API client with rate limiting
type Client struct {
	apiKey     string
	httpClient *http.Client

	// Simple rate limiter
	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

// NewClient creates a new Nexon API client
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
	req.Header.Set("x-nxopen-api-key", c.apiKey)

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
		return c.parseError(resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// parseError parses Nexon API error responses
func (c *Client) parseError(statusCode int, body []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Name != "" {
		return fmt.Errorf("%s: %s (HTTP %d)", apiErr.Error.Name, apiErr.Error.Message, statusCode)
	}

	// Fallback error messages for known error codes
	switch statusCode {
	case 400:
		return fmt.Errorf("잘못된 요청입니다 (HTTP 400): %s", string(body))
	case 403:
		return fmt.Errorf("권한이 없습니다 (HTTP 403)")
	case 429:
		return fmt.Errorf("API 호출량 초과 (HTTP 429)")
	case 500:
		return fmt.Errorf("서버 내부 오류 (HTTP 500)")
	case 503:
		return fmt.Errorf("API 점검 중 (HTTP 503)")
	default:
		return fmt.Errorf("API 오류: HTTP %d, body: %s", statusCode, string(body))
	}
}
