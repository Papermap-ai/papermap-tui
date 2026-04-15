package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/papermap/papermap-tui/internal/auth"
)

type InsightRequest struct {
	Prompt                  string         `json:"prompt"`
	WorkspaceID             string         `json:"workspace_id"`
	ChatID                  string         `json:"chat_id"`
	ReportID                string         `json:"report_id,omitempty"`
	RequestID               string         `json:"request_id,omitempty"`
	LLMModel                string         `json:"llm_model,omitempty"`
	UseSearch               *bool          `json:"use_search,omitempty"`
	AllowWorkspaceSwitching bool           `json:"allow_workspace_switching"`
	StreamExecution         bool           `json:"stream_execution"`
	DisplayPrompt           string         `json:"display_prompt,omitempty"`
	RundownMeta             map[string]any `json:"rundown_meta,omitempty"`
	InteractionSource       string         `json:"interaction_source,omitempty"`
}

type InsightResponse struct {
	LLMDataID           string         `json:"llm_data_id"`
	ResponseType        string         `json:"response_type"`
	TextResponse        string         `json:"text_response"`
	Status              string         `json:"status,omitempty"`
	Data                any            `json:"data"`
	Code                string         `json:"code,omitempty"`
	Error               bool           `json:"error"`
	Thoughts            string         `json:"thoughts,omitempty"`
	ThoughtLog          []any          `json:"thought_log,omitempty"`
	ChartType           string         `json:"chart_type,omitempty"`
	SchemaHints         map[string]any `json:"schema_hints,omitempty"`
	CompleteQueryPlan   string         `json:"complete_query_task_plan,omitempty"`
	VisualizationConfig map[string]any `json:"visualization_config,omitempty"`
	ProgressEvents      []any          `json:"progress_events,omitempty"`
	Raw                 map[string]any
}

func (r *InsightResponse) UnmarshalJSON(data []byte) error {
	type alias InsightResponse
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*r = InsightResponse(decoded)
	r.Raw = raw
	r.LLMDataID = firstString(decoded.LLMDataID, lookupString(raw, "llm_data_id"), lookupNestedString(raw, "data", "llm_data_id"))
	r.ResponseType = firstString(decoded.ResponseType, lookupString(raw, "response_type"), lookupNestedString(raw, "data", "response_type"))
	r.TextResponse = firstRawString(decoded.TextResponse, lookupRawString(raw, "text_response"), lookupNestedRawString(raw, "data", "text_response"))
	r.Status = firstString(decoded.Status, lookupString(raw, "status"), lookupNestedString(raw, "data", "status"))
	r.Code = firstRawString(decoded.Code, lookupRawString(raw, "code"), lookupNestedRawString(raw, "data", "code"))
	r.Thoughts = firstRawString(decoded.Thoughts, lookupRawString(raw, "thoughts"), lookupNestedRawString(raw, "data", "thoughts"))
	r.ChartType = firstString(decoded.ChartType, lookupString(raw, "chart_type"), lookupNestedString(raw, "data", "chart_type"))

	return nil
}

type InsightStreamRequest struct {
	RequestID string `json:"request_id,omitempty"`
	ChatID    string `json:"chat_id,omitempty"`
}

type InsightTable struct {
	Columns []string
	Rows    [][]string
}

type InsightMessage struct {
	Role    string
	Content string
	Table   *InsightTable
	Raw     map[string]any
}

type InsightStreamEvent struct {
	Type      string
	Text      string
	Error     string
	Done      bool
	RequestID string
	ChatID    string
	Table     *InsightTable
	Raw       map[string]any
}

type InsightStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	closed  bool
	event   string
	data    []string
	queued  []InsightStreamEvent
}

func GenerateRequestID() string {
	return fmt.Sprintf("chat_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%1_000_000)
}

func (c *Client) StartInsight(ctx context.Context, reqBody InsightRequest) (InsightResponse, error) {
	req, err := c.NewRequest(ctx, http.MethodPost, "/api/v1/analytics/charts/stream", reqBody)
	if err != nil {
		return InsightResponse{}, err
	}

	resp, err := c.DoStream(req)
	if err != nil {
		return InsightResponse{}, err
	}
	defer resp.Body.Close()

	decoded, err := decodeJSONResponse[InsightResponse](resp)
	if err != nil {
		return InsightResponse{}, err
	}

	return decoded, nil
}

func (c *Client) OpenInsightStream(ctx context.Context, reqBody InsightStreamRequest) (*InsightStream, error) {
	req, err := c.NewRequestWithHeaders(ctx, http.MethodPost, "/api/v1/analytics/requests/stream", reqBody, map[string]string{
		"Accept": "text/event-stream",
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.DoStream(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		return nil, checkResponseStatus(resp.StatusCode, resp.Status, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	return &InsightStream{
		body:    resp.Body,
		scanner: scanner,
	}, nil
}

func (c *Client) ConversationHistory(ctx context.Context, chatID string) ([]InsightMessage, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/analytics/chats/"+strings.TrimSpace(chatID)+"/conversations", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if err := checkResponseStatus(resp.StatusCode, resp.Status, body); err != nil {
		return nil, err
	}

	return decodeConversationHistory(body)
}

func (s *InsightStream) Next() (InsightStreamEvent, error) {
	if len(s.queued) > 0 {
		event := s.queued[0]
		s.queued = s.queued[1:]
		return event, nil
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			event, ok, err := decodeSSEEvent(s.event, s.data)
			s.event = ""
			s.data = s.data[:0]
			if err != nil {
				return InsightStreamEvent{}, err
			}
			if ok {
				return event, nil
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			s.event = value
		case "data":
			s.data = append(s.data, value)
		}
	}

	if err := s.scanner.Err(); err != nil {
		return InsightStreamEvent{}, fmt.Errorf("read stream: %w", err)
	}

	if len(s.data) > 0 || s.event != "" {
		event, ok, err := decodeSSEEvent(s.event, s.data)
		s.event = ""
		s.data = s.data[:0]
		if err != nil {
			return InsightStreamEvent{}, err
		}
		if ok {
			return event, nil
		}
	}

	return InsightStreamEvent{Done: true, Type: "done"}, io.EOF
}

func PrependInsightStream(stream *InsightStream, event InsightStreamEvent) *InsightStream {
	if stream == nil {
		return nil
	}
	stream.queued = append([]InsightStreamEvent{event}, stream.queued...)
	return stream
}

func (s *InsightStream) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if s.body == nil {
		return nil
	}
	return s.body.Close()
}

func decodeConversationHistory(body []byte) ([]InsightMessage, error) {
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err == nil {
		return normalizeConversationMessages(list), nil
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode conversation history: %w", err)
	}

	for _, key := range []string{"data", "conversations", "messages", "items"} {
		if items, ok := lookupSlice(raw, key); ok {
			return normalizeConversationMessages(items), nil
		}
		if nestedItems, ok := lookupNestedSlice(raw, key, "items"); ok {
			return normalizeConversationMessages(nestedItems), nil
		}
	}

	return nil, fmt.Errorf("conversation history missing messages")
}

func normalizeConversationMessages(items []map[string]any) []InsightMessage {
	messages := make([]InsightMessage, 0, len(items))
	for _, item := range items {
		role := firstString(
			lookupString(item, "role"),
			lookupString(item, "sender"),
			lookupString(item, "type"),
		)
		if role == "" {
			role = "assistant"
		}

		content := firstString(
			lookupString(item, "content"),
			lookupString(item, "text_response"),
			lookupString(item, "text"),
			lookupString(item, "message"),
			lookupNestedString(item, "content", "text"),
			lookupNestedString(item, "message", "content"),
		)

		messages = append(messages, InsightMessage{
			Role:    role,
			Content: content,
			Table:   extractTable(item),
			Raw:     item,
		})
	}

	return messages
}

func InsightMessagesFromResponse(user auth.User, prompt string, response InsightResponse) []InsightMessage {
	trimmedPrompt := strings.TrimSpace(prompt)
	messages := make([]InsightMessage, 0, 2)
	if trimmedPrompt != "" {
		role := "user"
		if strings.TrimSpace(user.Email) == "" {
			role = "user"
		}
		messages = append(messages, InsightMessage{Role: role, Content: trimmedPrompt})
	}

	content := firstRawString(response.TextResponse, response.Thoughts)
	if strings.TrimSpace(content) != "" || response.Data != nil {
		messages = append(messages, InsightMessage{
			Role:    "assistant",
			Content: strings.TrimSpace(content),
			Table:   extractTable(response.Raw),
			Raw:     response.Raw,
		})
	}

	return messages
}

func decodeSSEEvent(eventName string, dataLines []string) (InsightStreamEvent, bool, error) {
	payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
	if payload == "" {
		return InsightStreamEvent{}, false, nil
	}

	if payload == "[DONE]" {
		return InsightStreamEvent{Type: "done", Done: true}, true, nil
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		typeName := firstString(strings.TrimSpace(eventName), "message")
		return InsightStreamEvent{Type: typeName, Text: payload}, true, nil
	}

	typeName := strings.ToLower(firstString(
		strings.TrimSpace(eventName),
		lookupString(raw, "event"),
		lookupString(raw, "type"),
		lookupNestedString(raw, "data", "event"),
		lookupNestedString(raw, "data", "type"),
	))

	text := firstRawString(
		lookupRawString(raw, "text"),
		lookupRawString(raw, "content"),
		lookupRawString(raw, "chunk"),
		lookupRawString(raw, "delta"),
		lookupNestedRawString(raw, "data", "text"),
		lookupNestedRawString(raw, "data", "content"),
		lookupNestedRawString(raw, "data", "chunk"),
		lookupNestedRawString(raw, "payload", "text"),
		lookupNestedRawString(raw, "payload", "content"),
	)

	errorText := firstString(
		lookupString(raw, "error"),
		lookupNestedString(raw, "error", "message"),
	)

	done := typeName == "done" || typeName == "complete" || typeName == "completed" || typeName == "finish" || typeName == "finished" || lookupBool(raw, "done") || lookupBool(raw, "completed")
	if typeName == "error" && errorText == "" {
		errorText = firstString(lookupString(raw, "message"), lookupNestedString(raw, "data", "message"))
	}

	event := InsightStreamEvent{
		Type:      firstString(typeName, "message"),
		Text:      text,
		Error:     errorText,
		Done:      done,
		RequestID: firstString(lookupString(raw, "request_id"), lookupNestedString(raw, "data", "request_id"), lookupNestedString(raw, "meta", "request_id")),
		ChatID:    firstString(lookupString(raw, "chat_id"), lookupNestedString(raw, "data", "chat_id"), lookupNestedString(raw, "meta", "chat_id")),
		Table:     extractTable(raw),
		Raw:       raw,
	}

	if event.Error != "" {
		event.Type = "error"
	}

	return event, true, nil
}

func extractTable(raw map[string]any) *InsightTable {
	for _, candidate := range []map[string]any{raw} {
		if table := buildTable(candidate); table != nil {
			return table
		}
		for _, key := range []string{"data", "payload", "result", "table"} {
			nested, ok := candidate[key].(map[string]any)
			if !ok {
				continue
			}
			if table := buildTable(nested); table != nil {
				return table
			}
		}
	}

	return nil
}

func buildTable(raw map[string]any) *InsightTable {
	columnsValue, ok := raw["columns"]
	if !ok {
		return nil
	}
	rowsValue, ok := raw["rows"]
	if !ok {
		return nil
	}

	columns := toStringSlice(columnsValue)
	rows := toTableRows(rowsValue)
	if len(columns) == 0 || len(rows) == 0 {
		return nil
	}

	return &InsightTable{Columns: columns, Rows: rows}
}

func toStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, fmt.Sprint(item))
	}

	return result
}

func toTableRows(value any) [][]string {
	rowsAny, ok := value.([]any)
	if !ok {
		return nil
	}

	rows := make([][]string, 0, len(rowsAny))
	for _, rowAny := range rowsAny {
		cellsAny, ok := rowAny.([]any)
		if !ok {
			continue
		}
		row := make([]string, 0, len(cellsAny))
		for _, cell := range cellsAny {
			row = append(row, fmt.Sprint(cell))
		}
		rows = append(rows, row)
	}

	return rows
}

func lookupString(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue(value))
}

func lookupRawString(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	return stringValue(value)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func lookupNestedString(raw map[string]any, parent string, key string) string {
	nested, ok := raw[parent].(map[string]any)
	if !ok {
		return ""
	}
	return lookupString(nested, key)
}

func lookupNestedRawString(raw map[string]any, parent string, key string) string {
	nested, ok := raw[parent].(map[string]any)
	if !ok {
		return ""
	}
	return lookupRawString(nested, key)
}

func lookupBool(raw map[string]any, key string) bool {
	value, ok := raw[key]
	if !ok {
		return false
	}
	boolValue, ok := value.(bool)
	return ok && boolValue
}

func lookupSlice(raw map[string]any, key string) ([]map[string]any, bool) {
	value, ok := raw[key]
	if !ok {
		return nil, false
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	return toMapSlice(items), true
}

func lookupNestedSlice(raw map[string]any, parent string, key string) ([]map[string]any, bool) {
	nested, ok := raw[parent].(map[string]any)
	if !ok {
		return nil, false
	}
	return lookupSlice(nested, key)
}

func toMapSlice(items []any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		decoded, ok := item.(map[string]any)
		if ok {
			result = append(result, decoded)
		}
	}
	return result
}

func firstString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstRawString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
