package auth

import (
	"net/url"
	"testing"
)

func TestBuildLoginURLAllowsProductionFrontend(t *testing.T) {
	t.Setenv(allowUntrustedFrontendEnvKey, "")

	got, err := buildLoginURL("https://papermap.ai", "http://127.0.0.1:43123/callback", "abc123")
	if err != nil {
		t.Fatalf("buildLoginURL returned error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if u.Scheme != "https" {
		t.Fatalf("scheme = %q, want https", u.Scheme)
	}
	if u.Host != "papermap.ai" {
		t.Fatalf("host = %q, want papermap.ai", u.Host)
	}
	if u.Path != "/auth/login" {
		t.Fatalf("path = %q, want /auth/login", u.Path)
	}
	if q := u.Query().Get("cli_callback"); q != "http://127.0.0.1:43123/callback" {
		t.Fatalf("cli_callback = %q", q)
	}
	if q := u.Query().Get("state"); q != "abc123" {
		t.Fatalf("state = %q", q)
	}
}

func TestBuildLoginURLRejectsNonHTTPSByDefault(t *testing.T) {
	t.Setenv(allowUntrustedFrontendEnvKey, "")

	if _, err := buildLoginURL("http://papermap.ai", "http://127.0.0.1:43123/callback", "abc123"); err == nil {
		t.Fatal("expected error for non-https frontend url")
	}
}

func TestBuildLoginURLRejectsUntrustedHostByDefault(t *testing.T) {
	t.Setenv(allowUntrustedFrontendEnvKey, "")

	if _, err := buildLoginURL("https://evil.example", "http://127.0.0.1:43123/callback", "abc123"); err == nil {
		t.Fatal("expected error for untrusted frontend host")
	}
}

func TestBuildLoginURLAllowsUntrustedHostWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv(allowUntrustedFrontendEnvKey, "1")

	got, err := buildLoginURL("http://dev.internal", "http://127.0.0.1:43123/callback", "abc123")
	if err != nil {
		t.Fatalf("buildLoginURL returned error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if u.Host != "dev.internal" {
		t.Fatalf("host = %q, want dev.internal", u.Host)
	}
}
