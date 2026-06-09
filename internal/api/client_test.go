package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

func TestClientCheckRedirectStripsAuthOnCrossHost(t *testing.T) {
	t.Parallel()

	var redirectedAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/callback", http.StatusFound)
	}))
	defer origin.Close()

	client, err := api.NewClient(origin.URL, nil, staticTokenSource{token: "secret-token"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req, err := client.NewRequest(context.Background(), http.MethodGet, "/trigger", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if redirectedAuth != "" {
		t.Fatalf("expected Authorization to be stripped on cross-host redirect, got %q", redirectedAuth)
	}
}

func TestClientCheckRedirectPreservesAuthOnSameHost(t *testing.T) {
	t.Parallel()

	var redirectedAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/trigger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusFound)
	})
	mux.HandleFunc("/target", func(w http.ResponseWriter, r *http.Request) {
		redirectedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, staticTokenSource{token: "secret-token"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req, err := client.NewRequest(context.Background(), http.MethodGet, "/trigger", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if redirectedAuth != "Bearer secret-token" {
		t.Fatalf("expected Authorization preserved on same-host redirect, got %q", redirectedAuth)
	}
}

func TestClientNoRedirectPreservesAuth(t *testing.T) {
	t.Parallel()

	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, nil, staticTokenSource{token: "secret-token"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	req, err := client.NewRequest(context.Background(), http.MethodGet, "/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close response body: %v", cerr)
		}
	}()

	if receivedAuth != "Bearer secret-token" {
		t.Fatalf("expected Authorization on direct request, got %q", receivedAuth)
	}
}
