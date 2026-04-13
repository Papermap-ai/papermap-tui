package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type tokenClaims struct {
	Exp int64 `json:"exp"`
}

func ParseTokenExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("token must have three parts")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode token payload: %w", err)
	}

	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("decode token claims: %w", err)
	}
	if claims.Exp <= 0 {
		return time.Time{}, fmt.Errorf("token missing exp claim")
	}

	return time.Unix(claims.Exp, 0).UTC(), nil
}
