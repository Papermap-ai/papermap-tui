package api_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
)

type staticTokenSource struct {
	token string
}

type responseEnvelope[T any] struct {
	Message    string `json:"message"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Data       T      `json:"data"`
}

func (s staticTokenSource) AccessToken(context.Context) (string, error) {
	return s.token, nil
}

func TestClientLogin(t *testing.T) {
	t.Parallel()

	accessToken := jwtForTest(time.Unix(1_900_000_000, 0).UTC())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("login should not send auth header, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req api.LoginRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Email != "user@example.com" || req.Password != "hunter22" {
			t.Fatalf("unexpected request payload: %+v", req)
		}

		_ = json.NewEncoder(w).Encode(responseEnvelope[api.AuthTokens]{
			Message:    "Logged in successfully",
			Success:    true,
			StatusCode: http.StatusOK,
			Data: api.AuthTokens{
				AccessToken:  accessToken,
				RefreshToken: "refresh-token",
				TokenType:    "bearer",
				User: auth.User{
					UserID:    "user-1",
					Email:     "user@example.com",
					FirstName: "Test",
					LastName:  "User",
				},
			},
		})
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, staticTokenSource{token: "ignored"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	tokens, err := client.Login(context.Background(), "user@example.com", "hunter22")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	cred, err := tokens.ToCredentials(auth.Credentials{})
	if err != nil {
		t.Fatalf("to credentials: %v", err)
	}
	if cred.AccessToken != accessToken || cred.RefreshToken != "refresh-token" {
		t.Fatalf("unexpected credentials: %+v", cred)
	}
	if cred.User.Email != "user@example.com" {
		t.Fatalf("unexpected user: %+v", cred.User)
	}
	if cred.ExpiresAt.Unix() != 1_900_000_000 {
		t.Fatalf("unexpected expiry: %s", cred.ExpiresAt)
	}
}

func TestClientRefresh(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/refresh" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("refresh should not send auth header, got %q", got)
		}

		var req api.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.RefreshToken != "refresh-token" {
			t.Fatalf("unexpected refresh token: %q", req.RefreshToken)
		}

		_ = json.NewEncoder(w).Encode(responseEnvelope[api.AuthTokens]{
			Message:    "Token refreshed successfully",
			Success:    true,
			StatusCode: http.StatusOK,
			Data: api.AuthTokens{
				AccessToken:  jwtForTest(time.Unix(1_900_000_100, 0).UTC()),
				RefreshToken: "refresh-token-2",
				TokenType:    "bearer",
			},
		})
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, staticTokenSource{token: "ignored"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	tokens, err := client.Refresh(context.Background(), "refresh-token")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if tokens.RefreshToken != "refresh-token-2" {
		t.Fatalf("unexpected refresh response: %+v", tokens)
	}
}

func TestClientLogoutUsesBearerToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/logout" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, staticTokenSource{token: "access-token"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if err := client.Logout(context.Background()); err != nil {
		t.Fatalf("logout: %v", err)
	}
}

func TestClientCurrentUserUsesBearerToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/me" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("unexpected auth header: %q", got)
		}

		_ = json.NewEncoder(w).Encode(responseEnvelope[auth.User]{
			Message:    "User fetched successfully",
			Success:    true,
			StatusCode: http.StatusOK,
			Data: auth.User{
				UserID:    "user-1",
				Email:     "user@example.com",
				FirstName: "Test",
				LastName:  "User",
			},
		})
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, staticTokenSource{token: "access-token"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestCheckResponseStatusIncludesBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid credentials"))
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Login(context.Background(), "user@example.com", "bad-password")
	if err == nil || !strings.Contains(err.Error(), "invalid credentials") {
		t.Fatalf("expected error body in response, got %v", err)
	}
}

func jwtForTest(expiresAt time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(expiresAt.Unix(), 10) + `}`))
	return header + "." + payload + ".signature"
}
