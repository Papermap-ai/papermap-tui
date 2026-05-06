package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/app"
)

// confirmServer stands up an httptest server that records the body of
// every POST to /api/v1/analytics/requests/confirm and returns a happy
// envelope. Returns the server, a snapshot accessor, and a hit counter.
func confirmServer(t *testing.T) (*httptest.Server, func() (api.SubmitConfirmationRequest, int)) {
	t.Helper()

	var (
		mu      sync.Mutex
		hits    int
		gotBody api.SubmitConfirmationRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hits++
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		_ = json.NewEncoder(w).Encode(responseEnvelope[map[string]any]{
			Message: "ok",
			Success: true,
			Data: map[string]any{
				"request_id":      gotBody.RequestID,
				"confirmation_id": gotBody.ConfirmationID,
				"confirmed":       gotBody.Confirmed,
			},
		})
	}))
	t.Cleanup(server.Close)

	snapshot := func() (api.SubmitConfirmationRequest, int) {
		mu.Lock()
		defer mu.Unlock()
		return gotBody, hits
	}
	return server, snapshot
}

// TestConfirmationModalDenyOnEnter exercises the focused-deny path:
// the modal opens with deny focused, Enter submits, and the backend
// receives a `confirmed: false` payload.
func TestConfirmationModalDenyOnEnter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	const (
		requestID      = "req-conf-1"
		confirmationID = "conf-1"
	)

	server, snapshot := confirmServer(t)
	client, err := api.NewClient(server.URL, server.Client(), stubTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)

	model, _ = model.InjectConfirmationRequiredForTest(
		requestID, confirmationID,
		"Web Search", "Allow web search?", "Search the web for: golang",
		60, client,
	)

	pc, ok := model.PendingConfirmation()
	if !ok {
		t.Fatal("expected pendingConfirmation after inject")
	}
	if pc.AllowSelected {
		t.Fatal("expected deny to be the default focused button")
	}
	if pc.SecondsRemaining != 60 {
		t.Fatalf("expected secondsRemaining 60, got %d", pc.SecondsRemaining)
	}

	enter := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	next, cmd := model.Update(enter)
	model = next.(app.Model)

	pc, ok = model.PendingConfirmation()
	if !ok || !pc.Submitting {
		t.Fatalf("expected modal to remain open in submitting state, got pc=%+v ok=%v", pc, ok)
	}

	// Drive the cmd directly so we can inspect the resulting message.
	if cmd == nil {
		t.Fatal("expected cmd from submit")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("submit cmd produced no message")
	}

	// Feed the result back into Update; modal should clear.
	next, _ = model.Update(msg)
	model = next.(app.Model)
	if _, ok := model.PendingConfirmation(); ok {
		t.Fatal("expected pendingConfirmation cleared after successful submit")
	}

	// Validate backend payload.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, hits := snapshot()
		if hits >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	body, hits := snapshot()
	if hits != 1 {
		t.Fatalf("expected 1 backend hit, got %d", hits)
	}
	if body.RequestID != requestID || body.ConfirmationID != confirmationID || body.Confirmed {
		t.Fatalf("unexpected backend body: %+v", body)
	}
}

// TestConfirmationModalAllowAfterTab verifies Tab moves focus to Allow
// and Enter then submits with `confirmed: true`.
func TestConfirmationModalAllowAfterTab(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server, snapshot := confirmServer(t)
	client, err := api.NewClient(server.URL, server.Client(), stubTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)

	model, _ = model.InjectConfirmationRequiredForTest(
		"req-2", "conf-2", "Web Search", "Allow?", "", 30, client,
	)

	tab := tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	next, _ := model.Update(tab)
	model = next.(app.Model)

	pc, _ := model.PendingConfirmation()
	if !pc.AllowSelected {
		t.Fatal("expected Allow focused after Tab")
	}

	enter := tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	_, cmd := model.Update(enter)
	if cmd == nil {
		t.Fatal("expected submit cmd after Enter")
	}
	_ = cmd()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, hits := snapshot()
		if hits >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	body, _ := snapshot()
	if !body.Confirmed {
		t.Fatalf("expected confirmed:true, got %+v", body)
	}
}

// TestConfirmationModalAutoDenyOnTickToZero drives the countdown to
// zero and asserts the modal auto-submits with `confirmed: false`.
func TestConfirmationModalAutoDenyOnTickToZero(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server, snapshot := confirmServer(t)
	client, err := api.NewClient(server.URL, server.Client(), stubTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)

	const (
		requestID      = "req-auto"
		confirmationID = "conf-auto"
	)
	model, _ = model.InjectConfirmationRequiredForTest(
		requestID, confirmationID, "Web Search", "Allow?", "", 2, client,
	)

	// First tick: 2 -> 1, no submit yet.
	model, cmd := model.TickConfirmationForTest(requestID, confirmationID)
	if cmd == nil {
		t.Fatal("expected next tick scheduled")
	}
	if pc, ok := model.PendingConfirmation(); !ok || pc.SecondsRemaining != 1 {
		t.Fatalf("expected 1s remaining, got %+v ok=%v", pc, ok)
	}

	// Second tick: 1 -> 0, fires submit cmd.
	model, cmd = model.TickConfirmationForTest(requestID, confirmationID)
	if cmd == nil {
		t.Fatal("expected submit cmd at zero")
	}
	pc, ok := model.PendingConfirmation()
	if !ok || !pc.Submitting {
		t.Fatalf("expected modal in submitting state at zero, got %+v ok=%v", pc, ok)
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("submit cmd produced no message")
	}
	next, _ := model.Update(msg)
	model = next.(app.Model)
	if _, ok := model.PendingConfirmation(); ok {
		t.Fatal("expected modal cleared after auto-deny submit")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, hits := snapshot()
		if hits >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	body, hits := snapshot()
	if hits != 1 {
		t.Fatalf("expected 1 backend hit, got %d", hits)
	}
	if body.Confirmed {
		t.Fatal("expected auto-deny to send confirmed:false")
	}
}

// TestConfirmationStaleTickIgnored verifies that a tick whose ids do not
// match the active modal is dropped (e.g. left over from a previous
// request).
func TestConfirmationStaleTickIgnored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server, _ := confirmServer(t)
	client, err := api.NewClient(server.URL, server.Client(), stubTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	model, err := app.NewModel()
	if err != nil {
		t.Fatalf("NewModel returned error: %v", err)
	}
	model = model.SetAuthenticatedForTest().SeedChatForTest(80, 24)
	model, _ = model.InjectConfirmationRequiredForTest(
		"req-fresh", "conf-fresh", "Web Search", "Allow?", "", 5, client,
	)

	model, cmd := model.TickConfirmationForTest("req-stale", "conf-stale")
	if cmd != nil {
		t.Fatal("stale tick should not schedule another tick")
	}
	if pc, _ := model.PendingConfirmation(); pc.SecondsRemaining != 5 {
		t.Fatalf("expected countdown unchanged on stale tick, got %d", pc.SecondsRemaining)
	}
}
