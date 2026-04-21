package auth_test

import (
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/papermap/papermap-tui/internal/auth"
)

func TestParseTokenExpiry(t *testing.T) {
	t.Parallel()

	expiresAt := time.Unix(1_900_000_000, 0).UTC()
	token := jwtForTest(expiresAt.Unix())

	got, err := auth.ParseTokenExpiry(token)
	if err != nil {
		t.Fatalf("ParseTokenExpiry returned error: %v", err)
	}

	if !got.Equal(expiresAt) {
		t.Fatalf("unexpected expiry: got %s want %s", got, expiresAt)
	}
}

func TestParseTokenExpiryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
	}{
		{name: "invalid parts", token: "not-a-jwt"},
		{name: "invalid payload", token: "a.invalid.signature"},
		{name: "missing exp", token: jwtWithoutExp()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := auth.ParseTokenExpiry(tc.token); err == nil {
				t.Fatalf("expected error for token %q", tc.token)
			}
		})
	}
}

func jwtForTest(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(exp, 10) + `}`))
	return header + "." + payload + ".signature"
}

func jwtWithoutExp() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user-1"}`))
	return header + "." + payload + ".signature"
}
