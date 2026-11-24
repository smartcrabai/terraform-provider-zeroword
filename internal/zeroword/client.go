// Package zeroword is a minimal client for the Zeroword Management API
// (https://docs.zeroword.smartcrab.ai/management-api).
package zeroword

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the production Management API.
const DefaultBaseURL = "https://api.zeroword.smartcrab.ai/v1"

// Client calls the Management API with one Management API key.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	userAgent  string
}

func NewClient(baseURL, apiKey, userAgent string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		// Terraform passes contexts without a deadline; never hang on one request.
		httpClient: &http.Client{Timeout: time.Minute},
		userAgent:  userAgent,
	}
}

// APIError is an RFC 9457 Problem Details response.
type APIError struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	RequestID string `json:"request_id"`
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("zeroword: HTTP %d %s", e.Status, e.Code)
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	if e.RequestID != "" {
		msg += " (request_id " + e.RequestID + ")"
	}
	return msg
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	apiErr, ok := errors.AsType[*APIError](err)
	return ok && apiErr.Status == http.StatusNotFound
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("zeroword: encode %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zeroword: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("zeroword: read %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 300 {
		apiErr := &APIError{Status: resp.StatusCode}
		_ = json.Unmarshal(payload, apiErr)
		apiErr.Status = resp.StatusCode
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("zeroword: decode %s %s: %w", method, path, err)
	}
	return nil
}

func get[T any](ctx context.Context, c *Client, path string) (*T, error) {
	var out T
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func send[T any](ctx context.Context, c *Client, method, path string, body any) (*T, error) {
	var out T
	if err := c.do(ctx, method, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func id(path, value string) string { return path + "/" + url.PathEscape(value) }
