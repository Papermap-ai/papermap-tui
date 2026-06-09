package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/papermap/papermap-tui/internal/auth"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// CLITokenRequest exchanges a one-time CLI authorization code (minted by
// the frontend after a successful browser login) for normal access and
// refresh tokens. State must match the value the TUI generated when it
// started the browser login flow.
type CLITokenRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type AuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	User         auth.User `json:"user"`
}

type responseEnvelope[T any] struct {
	Message    string `json:"message"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Data       T      `json:"data"`
}

func (t AuthTokens) ToCredentials(existing auth.Credentials) (auth.Credentials, error) {
	cred := existing
	cred.AccessToken = strings.TrimSpace(t.AccessToken)
	cred.RefreshToken = strings.TrimSpace(t.RefreshToken)
	if t.User != (auth.User{}) {
		cred.User = t.User
	}

	if cred.AccessToken == "" {
		return auth.Credentials{}, fmt.Errorf("auth response missing access token")
	}
	if cred.RefreshToken == "" {
		return auth.Credentials{}, fmt.Errorf("auth response missing refresh token")
	}

	expiresAt, err := auth.ParseTokenExpiry(cred.AccessToken)
	if err != nil {
		return auth.Credentials{}, fmt.Errorf("parse access token expiry: %w", err)
	}

	cred.ExpiresAt = expiresAt
	return cred, nil
}

func (c *Client) Login(ctx context.Context, email string, password string) (AuthTokens, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/auth/login", LoginRequest{
		Email:    strings.TrimSpace(email),
		Password: password,
	}, false)
	if err != nil {
		return AuthTokens{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return AuthTokens{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[AuthTokens](resp)
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (AuthTokens, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/auth/refresh", RefreshRequest{
		RefreshToken: strings.TrimSpace(refreshToken),
	}, false)
	if err != nil {
		return AuthTokens{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return AuthTokens{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[AuthTokens](resp)
}

// ExchangeCLICode swaps a one-time CLI authorization code (received via
// the localhost browser-login callback) for normal access and refresh
// tokens. The request is sent unauthenticated; the backend authenticates
// the caller via the code itself.
func (c *Client) ExchangeCLICode(ctx context.Context, code, state string) (AuthTokens, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/auth/cli/token", CLITokenRequest{
		Code:  strings.TrimSpace(code),
		State: strings.TrimSpace(state),
	}, false)
	if err != nil {
		return AuthTokens{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return AuthTokens{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[AuthTokens](resp)
}

func (c *Client) Logout(ctx context.Context) error {
	req, err := c.NewRequest(ctx, http.MethodPost, "/api/v1/auth/logout", nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	return checkResponseStatus(resp.StatusCode, resp.Status, body)
}

func (c *Client) CurrentUser(ctx context.Context) (auth.User, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/users/me", nil)
	if err != nil {
		return auth.User{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return auth.User{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[auth.User](resp)
}

func decodeJSONResponse[T any](resp *http.Response) (T, error) {
	var zero T
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}

	if err := checkResponseStatus(resp.StatusCode, resp.Status, body); err != nil {
		return zero, err
	}

	if data, ok, err := decodeEnvelopeData[T](body); err != nil {
		return zero, err
	} else if ok {
		return data, nil
	}

	var value T
	if err := json.Unmarshal(body, &value); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}

	return value, nil
}

func decodeEnvelopeData[T any](body []byte) (T, bool, error) {
	var zero T

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return zero, false, nil
	}

	if _, ok := raw["data"]; !ok {
		return zero, false, nil
	}

	var envelope responseEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return zero, true, fmt.Errorf("decode wrapped response: %w", err)
	}

	return envelope.Data, true, nil
}

func checkResponseStatus(statusCode int, status string, body []byte) error {
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return nil
	}

	message := extractResponseMessage(body)
	if message == "" {
		message = status
	}

	return fmt.Errorf("api request failed: %s", message)
}

func extractResponseMessage(body []byte) string {
	message := strings.TrimSpace(string(body))

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return message
	}

	if value, ok := raw["message"]; ok {
		var decoded string
		if err := json.Unmarshal(value, &decoded); err == nil && strings.TrimSpace(decoded) != "" {
			return decoded
		}
	}

	return message
}
