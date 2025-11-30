// Package client provides an HTTP client for the SnitchDNS API with retry logic and timeout support.
package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	emptyJSON = "{}"
)

// Client is the SnitchDNS API client
type Client struct {
	BaseURL      string
	APIKey       string
	HTTPClient   *http.Client
	UserAgent    string
	MaxRetries   int
	RetryWaitMin time.Duration
	RetryWaitMax time.Duration
	DebugLogging bool
}

// NewClient creates a new SnitchDNS API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent:    "terraform-provider-snitchdns/dev",
		MaxRetries:   3,
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 30 * time.Second,
		DebugLogging: false,
	}
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	return c.doRequestWithContext(context.Background(), method, path, body)
}

// doRequestWithContext performs an HTTP request with authentication and context
func (c *Client) doRequestWithContext(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var jsonData []byte
	var err error

	if body != nil {
		jsonData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter
			wait := c.calculateBackoff(attempt)

			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		respBody, statusCode, err := c.executeRequest(ctx, method, path, jsonData)
		if err != nil {
			// Check if error is context-related (don't retry)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}

		// Success
		if statusCode >= 200 && statusCode < 300 {
			return respBody, nil
		}

		// 4xx errors are not retried (client errors)
		if statusCode >= 400 && statusCode < 500 {
			return nil, fmt.Errorf("API request failed with status %d: %s", statusCode, string(respBody))
		}

		// 5xx errors are retried
		lastErr = fmt.Errorf("API request failed with status %d: %s", statusCode, string(respBody))
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.MaxRetries, lastErr)
}

// executeRequest performs a single HTTP request attempt
func (c *Client) executeRequest(ctx context.Context, method, path string, jsonData []byte) (respBody []byte, statusCode int, err error) {
	var reqBody io.Reader
	if jsonData != nil {
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-SnitchDNS-Auth", c.APIKey)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if jsonData != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't override the main error
			_ = closeErr
		}
	}()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		statusCode = resp.StatusCode
		err = fmt.Errorf("failed to read response body: %w", err)
		return
	}

	statusCode = resp.StatusCode
	return
}

// calculateBackoff calculates the backoff duration with exponential backoff and jitter
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: min * (2 ^ attempt)
	backoff := float64(c.RetryWaitMin) * math.Pow(2, float64(attempt-1))

	// Cap at max
	if backoff > float64(c.RetryWaitMax) {
		backoff = float64(c.RetryWaitMax)
	}

	// Add jitter (±25%) using crypto/rand for security
	jitter := backoff * 0.25
	randomFactor := secureRandomFloat()
	backoff = backoff - jitter + (randomFactor * jitter * 2)

	return time.Duration(backoff)
}

// secureRandomFloat returns a cryptographically secure random float64 between 0 and 1
func secureRandomFloat() float64 {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to using timestamp if crypto/rand fails (should never happen)
		return float64(time.Now().UnixNano()%1000) / 1000.0
	}
	// Convert bytes to uint64 and normalize to [0, 1)
	return float64(binary.BigEndian.Uint64(b[:])) / float64(1<<64)
}

// Zone represents a DNS zone
type Zone struct {
	ID         int      `json:"id,omitempty"`
	UserID     int      `json:"user_id,omitempty"`
	Domain     string   `json:"domain"`
	Active     bool     `json:"active"`
	CatchAll   bool     `json:"catch_all"`
	Forwarding bool     `json:"forwarding"`
	Regex      bool     `json:"regex"`
	Master     bool     `json:"master,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	CreatedAt  string   `json:"created_at,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

// CreateZoneRequest is the request body for creating a zone
type CreateZoneRequest struct {
	Domain     string `json:"domain"`
	Active     bool   `json:"active"`
	CatchAll   bool   `json:"catch_all"`
	Forwarding bool   `json:"forwarding"`
	Regex      bool   `json:"regex"`
	Master     bool   `json:"master"`
	Tags       string `json:"tags"`
}

// UpdateZoneRequest is the request body for updating a zone
type UpdateZoneRequest struct {
	Domain     *string `json:"domain,omitempty"`
	Active     *bool   `json:"active,omitempty"`
	CatchAll   *bool   `json:"catch_all,omitempty"`
	Forwarding *bool   `json:"forwarding,omitempty"`
	Regex      *bool   `json:"regex,omitempty"`
	Tags       *string `json:"tags,omitempty"`
}

// CreateZone creates a new DNS zone
func (c *Client) CreateZone(req CreateZoneRequest) (*Zone, error) {
	respBody, err := c.doRequest("POST", "/zones", req)
	if err != nil {
		return nil, err
	}

	var zone Zone
	if err := json.Unmarshal(respBody, &zone); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &zone, nil
}

// GetZone retrieves a zone by ID
func (c *Client) GetZone(id string) (*Zone, error) {
	return c.GetZoneWithContext(context.Background(), id)
}

// GetZoneWithContext retrieves a zone by ID with context
func (c *Client) GetZoneWithContext(ctx context.Context, id string) (*Zone, error) {
	respBody, err := c.doRequestWithContext(ctx, "GET", fmt.Sprintf("/zones/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var zone Zone
	if err := json.Unmarshal(respBody, &zone); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &zone, nil
}

// UpdateZone updates an existing zone
func (c *Client) UpdateZone(id string, req UpdateZoneRequest) (*Zone, error) {
	respBody, err := c.doRequest("POST", fmt.Sprintf("/zones/%s", id), req)
	if err != nil {
		return nil, err
	}

	var zone Zone
	if err := json.Unmarshal(respBody, &zone); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &zone, nil
}

// DeleteZone deletes a zone
func (c *Client) DeleteZone(id string) error {
	_, err := c.doRequest("DELETE", fmt.Sprintf("/zones/%s", id), nil)
	return err
}

// Record represents a DNS record
type Record struct {
	ID                 int    `json:"id,omitempty"`
	ZoneID             int    `json:"zone_id,omitempty"`
	Active             bool   `json:"active"`
	Class              string `json:"cls"`
	Type               string `json:"type"`
	TTL                int    `json:"ttl"`
	DataRaw            string `json:"data"`
	IsConditional      bool   `json:"is_conditional"`
	ConditionalCount   int    `json:"conditional_count,omitempty"`
	ConditionalLimit   int    `json:"conditional_limit,omitempty"`
	ConditionalReset   bool   `json:"conditional_reset,omitempty"`
	ConditionalDataRaw string `json:"conditional_data,omitempty"`

	// Parsed versions (not from JSON)
	Data            map[string]interface{} `json:"-"`
	ConditionalData map[string]interface{} `json:"-"`
}

// CreateRecordRequest is the request body for creating a record
type CreateRecordRequest struct {
	Active           bool                   `json:"active"`
	Class            string                 `json:"class"`
	Type             string                 `json:"type"`
	TTL              int                    `json:"ttl"`
	Data             map[string]interface{} `json:"data"`
	IsConditional    bool                   `json:"is_conditional"`
	ConditionalCount int                    `json:"conditional_count"`
	ConditionalLimit int                    `json:"conditional_limit"`
	ConditionalReset bool                   `json:"conditional_reset"`
	ConditionalData  map[string]interface{} `json:"conditional_data"`
}

// UpdateRecordRequest is the request body for updating a record
type UpdateRecordRequest struct {
	Active           *bool                  `json:"active,omitempty"`
	Class            *string                `json:"class,omitempty"`
	Type             *string                `json:"type,omitempty"`
	TTL              *int                   `json:"ttl,omitempty"`
	Data             map[string]interface{} `json:"data,omitempty"`
	IsConditional    *bool                  `json:"is_conditional,omitempty"`
	ConditionalCount *int                   `json:"conditional_count,omitempty"`
	ConditionalLimit *int                   `json:"conditional_limit,omitempty"`
	ConditionalReset *bool                  `json:"conditional_reset,omitempty"`
	ConditionalData  map[string]interface{} `json:"conditional_data,omitempty"`
}

// CreateRecord creates a new DNS record
func (c *Client) CreateRecord(zoneID string, req CreateRecordRequest) (*Record, error) {
	respBody, err := c.doRequest("POST", fmt.Sprintf("/zones/%s/records", zoneID), req)
	if err != nil {
		return nil, err
	}

	var record Record
	if err := json.Unmarshal(respBody, &record); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse the data JSON string
	if record.DataRaw != "" {
		if err := json.Unmarshal([]byte(record.DataRaw), &record.Data); err != nil {
			return nil, fmt.Errorf("failed to parse data field: %w", err)
		}
	}

	// Parse the conditional_data JSON string
	if record.ConditionalDataRaw != "" && record.ConditionalDataRaw != emptyJSON {
		if err := json.Unmarshal([]byte(record.ConditionalDataRaw), &record.ConditionalData); err != nil {
			return nil, fmt.Errorf("failed to parse conditional_data field: %w", err)
		}
	}

	return &record, nil
}

// GetRecord retrieves a record by zone ID and record ID
func (c *Client) GetRecord(zoneID, recordID string) (*Record, error) {
	respBody, err := c.doRequest("GET", fmt.Sprintf("/zones/%s/records/%s", zoneID, recordID), nil)
	if err != nil {
		return nil, err
	}

	var record Record
	if err := json.Unmarshal(respBody, &record); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse the data JSON string
	if record.DataRaw != "" {
		if err := json.Unmarshal([]byte(record.DataRaw), &record.Data); err != nil {
			return nil, fmt.Errorf("failed to parse data field: %w", err)
		}
	}

	// Parse the conditional_data JSON string
	if record.ConditionalDataRaw != "" && record.ConditionalDataRaw != emptyJSON {
		if err := json.Unmarshal([]byte(record.ConditionalDataRaw), &record.ConditionalData); err != nil {
			return nil, fmt.Errorf("failed to parse conditional_data field: %w", err)
		}
	}

	return &record, nil
}

// UpdateRecord updates an existing DNS record
func (c *Client) UpdateRecord(zoneID, recordID string, req UpdateRecordRequest) (*Record, error) {
	respBody, err := c.doRequest("POST", fmt.Sprintf("/zones/%s/records/%s", zoneID, recordID), req)
	if err != nil {
		return nil, err
	}

	var record Record
	if err := json.Unmarshal(respBody, &record); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse the data JSON string
	if record.DataRaw != "" {
		if err := json.Unmarshal([]byte(record.DataRaw), &record.Data); err != nil {
			return nil, fmt.Errorf("failed to parse data field: %w", err)
		}
	}

	// Parse the conditional_data JSON string
	if record.ConditionalDataRaw != "" && record.ConditionalDataRaw != emptyJSON {
		if err := json.Unmarshal([]byte(record.ConditionalDataRaw), &record.ConditionalData); err != nil {
			return nil, fmt.Errorf("failed to parse conditional_data field: %w", err)
		}
	}

	return &record, nil
}

// DeleteRecord deletes a DNS record
func (c *Client) DeleteRecord(zoneID, recordID string) error {
	_, err := c.doRequest("DELETE", fmt.Sprintf("/zones/%s/records/%s", zoneID, recordID), nil)
	return err
}
