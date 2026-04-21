package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/papermap/papermap-tui/internal/api"
)

type insightTokenSource struct{}

func (insightTokenSource) AccessToken(context.Context) (string, error) {
	return "test-token", nil
}

func TestStartInsightAndStream(t *testing.T) {
	t.Parallel()

	var chartsAuth string
	var streamAuth string
	var chartRequest api.InsightRequest
	var streamRequest api.InsightStreamRequest
	var rawChartRequest map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/analytics/charts/stream":
			chartsAuth = r.Header.Get("Authorization")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read chart request: %v", err)
			}
			if err := json.Unmarshal(body, &chartRequest); err != nil {
				t.Fatalf("decode chart request: %v", err)
			}
			if err := json.Unmarshal(body, &rawChartRequest); err != nil {
				t.Fatalf("decode raw chart request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(responseEnvelope[map[string]any]{
				Message:    "ok",
				Success:    true,
				StatusCode: http.StatusOK,
				Data: map[string]any{
					"llm_data_id":   "llm-123",
					"response_type": "text",
					"text_response": "final response",
					"status":        "success",
				},
			})
		case "/api/v1/analytics/requests/stream":
			streamAuth = r.Header.Get("Authorization")
			if err := json.NewDecoder(r.Body).Decode(&streamRequest); err != nil {
				t.Fatalf("decode stream request: %v", err)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			// Real backend emits progress events (phase_update, agent_thought,
			// tool_*) plus a final `complete` sentinel. The client should
			// extract phase/message for phase_update and detect done on
			// complete. No SSE event carries the final answer text.
			_, _ = io.WriteString(w, strings.Join([]string{
				"event: phase_update",
				`data: {"type":"phase_update","phase":"analyzing","message":"Analyzing data...","request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: agent_thought",
				`data: {"type":"agent_thought","content":"thinking...","is_complete":false,"request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: complete",
				`data: {"type":"complete","status":"success","request_id":"req-123","chat_id":"chat-123"}`,
				"",
			}, "\n"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	requestID := api.GenerateRequestID()
	response, err := client.StartInsight(context.Background(), api.InsightRequest{
		Prompt:          "hello",
		RequestID:       requestID,
		StreamExecution: true,
	})
	if err != nil {
		t.Fatalf("StartInsight returned error: %v", err)
	}
	if response.LLMDataID != "llm-123" || response.TextResponse != "final response" {
		t.Fatalf("unexpected insight response: %+v", response)
	}

	stream, err := client.OpenInsightStream(context.Background(), api.InsightStreamRequest{RequestID: requestID})
	if err != nil {
		t.Fatalf("OpenInsightStream returned error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	first, err := stream.Next()
	if err != nil {
		t.Fatalf("first Next returned error: %v", err)
	}
	if first.Type != "phase_update" || first.Phase != "analyzing" || first.Message != "Analyzing data..." {
		t.Fatalf("unexpected first event: %+v", first)
	}

	second, err := stream.Next()
	if err != nil {
		t.Fatalf("second Next returned error: %v", err)
	}
	// agent_thought carries reasoning; the client deliberately does NOT
	// surface its content as text.
	if second.Type != "agent_thought" || second.Message != "" {
		t.Fatalf("unexpected second event: %+v", second)
	}

	third, err := stream.Next()
	if err != nil {
		t.Fatalf("third Next returned error: %v", err)
	}
	if !third.Done {
		t.Fatalf("expected done event, got %+v", third)
	}

	if chartsAuth != "Bearer test-token" {
		t.Fatalf("expected auth header on charts request, got %q", chartsAuth)
	}
	if chartRequest.RequestID != requestID || !chartRequest.StreamExecution {
		t.Fatalf("unexpected chart request payload: %+v", chartRequest)
	}
	if _, ok := rawChartRequest["workspace_id"]; !ok {
		t.Fatalf("expected workspace_id key in raw request body, got %+v", rawChartRequest)
	}
	if _, ok := rawChartRequest["chat_id"]; !ok {
		t.Fatalf("expected chat_id key in raw request body, got %+v", rawChartRequest)
	}
	if streamAuth != "Bearer test-token" {
		t.Fatalf("expected auth header on stream request, got %q", streamAuth)
	}
	if streamRequest.RequestID != requestID {
		t.Fatalf("unexpected stream request payload: %+v", streamRequest)
	}
}
