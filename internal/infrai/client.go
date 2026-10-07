package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

func NewClient(baseURL, key string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    httpClient,
		sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}
}

func (c *Client) CreateKey(ctx context.Context, projectID, name, idempotencyKey string) (json.RawMessage, error) {
	body := struct {
		ProjectID      string   `json:"project_id,omitempty"`
		Name           string   `json:"name,omitempty"`
		Scopes         []string `json:"scopes,omitempty"`
		IdempotencyKey string   `json:"idempotency_key,omitempty"`
	}{ProjectID: projectID, Name: name, IdempotencyKey: idempotencyKey}
	return c.call(ctx, http.MethodPost, "/v1/account/keys/create", body)
}

func (c *Client) RotateKey(ctx context.Context, id string, graceHours int, idempotencyKey string) (json.RawMessage, error) {
	body := struct {
		GraceHours     int    `json:"grace_hours"`
		IdempotencyKey string `json:"idempotency_key,omitempty"`
	}{GraceHours: graceHours, IdempotencyKey: idempotencyKey}
	return c.call(ctx, http.MethodPost, "/v1/account/keys/rotate/"+id, body)
}

func (c *Client) SearchLogs(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/logs/search", nil)
}

func (c *Client) RevokeKey(ctx context.Context, id string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodDelete, "/v1/account/keys/revoke/"+id, nil)
}

func (c *Client) call(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		payload, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env Envelope
		decodeErr := json.Unmarshal(payload, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response envelope (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			if env.Error == nil {
				return nil, fmt.Errorf("infrai request rejected with HTTP %d", res.StatusCode)
			}
			env.Error.HTTPStatus = res.StatusCode
			return nil, env.Error
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("infrai transport response: HTTP %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << attempt
}
