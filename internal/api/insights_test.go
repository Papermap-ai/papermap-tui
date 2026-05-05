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
			// tool_*) plus a final `complete` sentinel. The client extracts
			// phase/message for phase_update, populates trace fields for
			// reasoning and tool lifecycle events, and detects done on
			// complete. No SSE event carries the final answer text.
			_, _ = io.WriteString(w, strings.Join([]string{
				"event: phase_update",
				`data: {"type":"phase_update","phase":"analyzing","message":"Analyzing data...","request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: agent_thought",
				`data: {"type":"agent_thought","content":"thinking...","is_complete":false,"iteration":1,"request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: tool_call_announced",
				`data: {"type":"tool_call_announced","tool_name":"sql_query","tool_display_name":"SQL Query","iteration":1,"tool_call_id":"call-1","request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: tool_call_args_complete",
				`data: {"type":"tool_call_args_complete","tool_name":"sql_query","tool_display_name":"SQL Query","tool_call_id":"call-1","full_args":{"query":"SELECT 1"},"private_args":false,"request_id":"req-123","chat_id":"chat-123"}`,
				"",
				"event: tool_call_complete",
				`data: {"type":"tool_call_complete","tool_name":"sql_query","tool_display_name":"SQL Query","tool_call_id":"call-1","status":"success","duration_ms":42.5,"result_preview":"14 rows","request_id":"req-123","chat_id":"chat-123"}`,
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
	// agent_thought now carries the reasoning content as a trace field
	// so the chat layer can render Alan's thinking. The legacy Message
	// field stays empty: only phase_update populates Message.
	if second.Type != "agent_thought" || second.Message != "" {
		t.Fatalf("unexpected second event: %+v", second)
	}
	if second.Content != "thinking..." || second.Iteration != 1 || second.IsComplete {
		t.Fatalf("unexpected agent_thought trace fields: %+v", second)
	}

	announced, err := stream.Next()
	if err != nil {
		t.Fatalf("tool_call_announced Next returned error: %v", err)
	}
	if announced.Type != "tool_call_announced" || announced.ToolCallID != "call-1" ||
		announced.ToolName != "sql_query" || announced.ToolDisplayName != "SQL Query" {
		t.Fatalf("unexpected tool_call_announced event: %+v", announced)
	}

	argsComplete, err := stream.Next()
	if err != nil {
		t.Fatalf("tool_call_args_complete Next returned error: %v", err)
	}
	if argsComplete.Type != "tool_call_args_complete" || argsComplete.ToolCallID != "call-1" {
		t.Fatalf("unexpected tool_call_args_complete event: %+v", argsComplete)
	}

	toolDone, err := stream.Next()
	if err != nil {
		t.Fatalf("tool_call_complete Next returned error: %v", err)
	}
	if toolDone.Type != "tool_call_complete" || toolDone.Status != "success" ||
		toolDone.DurationMS != 42.5 || toolDone.ResultPreview != "14 rows" {
		t.Fatalf("unexpected tool_call_complete event: %+v", toolDone)
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

func TestCancelInsight(t *testing.T) {
	t.Parallel()

	var (
		gotPath    string
		gotMethod  string
		gotAuth    string
		gotRequest api.CancelInsightRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read cancel request: %v", err)
		}
		if err := json.Unmarshal(body, &gotRequest); err != nil {
			t.Fatalf("decode cancel request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(responseEnvelope[map[string]any]{
			Message:    "cancelled",
			Success:    true,
			StatusCode: http.StatusOK,
			Data: map[string]any{
				"request_id": gotRequest.RequestID,
				"chat_id":    "chat-abc",
				"status":     "cancelled",
			},
		})
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	resp, err := client.CancelInsight(context.Background(), api.CancelInsightRequest{
		RequestID: "req-cancel-1",
	})
	if err != nil {
		t.Fatalf("CancelInsight returned error: %v", err)
	}

	if gotPath != "/api/v1/analytics/charts/cancel" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotRequest.RequestID != "req-cancel-1" {
		t.Fatalf("unexpected request_id: %q", gotRequest.RequestID)
	}
	if gotRequest.Reason != "user_cancelled" {
		t.Fatalf("expected reason to default to user_cancelled, got %q", gotRequest.Reason)
	}
	if resp.RequestID != "req-cancel-1" || resp.ChatID != "chat-abc" || resp.Status != "cancelled" {
		t.Fatalf("unexpected cancel response: %+v", resp)
	}
}

func TestCancelInsightRequiresRequestID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called when request_id is empty")
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if _, err := client.CancelInsight(context.Background(), api.CancelInsightRequest{}); err == nil {
		t.Fatal("expected error when request_id is missing")
	}
}

func TestInsightStreamConfirmationRequired(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			"event: confirmation_required",
			`data: {"type":"confirmation_required","confirmation_id":"conf-9","tool_display_name":"Web Search","message":"Allow web search?","action_description":"Search the web for: golang sse","timeout_seconds":60,"request_id":"req-9","chat_id":"chat-9"}`,
			"",
			"event: complete",
			`data: {"type":"complete","status":"success","request_id":"req-9","chat_id":"chat-9"}`,
			"",
		}, "\n"))
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, server.Client(), insightTokenSource{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	stream, err := client.OpenInsightStream(context.Background(), api.InsightStreamRequest{RequestID: "req-9"})
	if err != nil {
		t.Fatalf("OpenInsightStream returned error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	event, err := stream.Next()
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	if event.Type != "confirmation_required" {
		t.Fatalf("expected confirmation_required, got %q", event.Type)
	}
	if event.ConfirmationID != "conf-9" {
		t.Fatalf("expected confirmation_id conf-9, got %q", event.ConfirmationID)
	}
	if event.ToolDisplayName != "Web Search" {
		t.Fatalf("expected tool display name Web Search, got %q", event.ToolDisplayName)
	}
	if event.Message != "Allow web search?" {
		t.Fatalf("expected message populated, got %q", event.Message)
	}
	if event.ActionDescription != "Search the web for: golang sse" {
		t.Fatalf("unexpected action description: %q", event.ActionDescription)
	}
	if event.TimeoutSeconds != 60 {
		t.Fatalf("expected timeout 60, got %d", event.TimeoutSeconds)
	}
	if event.RequestID != "req-9" {
		t.Fatalf("expected request_id req-9, got %q", event.RequestID)
	}
}
