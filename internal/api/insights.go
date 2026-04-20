package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	// RawDataJSON holds the raw JSON bytes of the `data` field (or the
	// nested `data.data` field when the backend wraps responses in an
	// envelope). Preserved so renderers can re-parse with json.Decoder to
	// recover JSON object key insertion order, which is lost when
	// decoding into map[string]any.
	RawDataJSON json.RawMessage
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
	r.RawDataJSON = extractRawDataField(data)

	return nil
}

// extractRawDataField returns the raw JSON bytes of the `data` field from
// the top-level object. If the top-level `data` is itself an object that
// contains a nested `data` array (envelope shape), it returns that nested
// array's bytes instead. Returns nil when no usable `data` field exists.
func extractRawDataField(body []byte) json.RawMessage {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil
	}
	dataBytes, ok := top["data"]
	if !ok {
		return nil
	}

	trimmed := bytes.TrimSpace(dataBytes)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	// If the outer `data` is an object, peek for a nested `data` array.
	if trimmed[0] == '{' {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &nested); err == nil {
			if inner, ok := nested["data"]; ok {
				innerTrim := bytes.TrimSpace(inner)
				if len(innerTrim) > 0 && innerTrim[0] == '[' {
					return inner
				}
			}
		}
		return nil
	}

	if trimmed[0] != '[' {
		return nil
	}
	return dataBytes
}

type InsightStreamRequest struct {
	RequestID string `json:"request_id,omitempty"`
	ChatID    string `json:"chat_id,omitempty"`
}

type InsightTable struct {
	Columns []string
	Rows    [][]string
}

// TileData represents a single-metric tile rendered as a card. Label is the
// derived column/key name; Value is the formatted scalar value as a string.
type TileData struct {
	Label string
	Value string
}

type InsightStreamEvent struct {
	Type      string
	Phase     string
	Message   string
	Error     string
	Done      bool
	RequestID string
	ChatID    string
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

// GetChart fetches a single chart record by its llm_data id. This is used
// for lazy-loading chart payloads (chart_type + data) for messages in
// conversation history, where the history endpoint strips that data to keep
// list responses small.
func (c *Client) GetChart(ctx context.Context, llmDataID string) (InsightResponse, error) {
	id := strings.TrimSpace(llmDataID)
	if id == "" {
		return InsightResponse{}, fmt.Errorf("llm_data_id is required")
	}

	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/analytics/charts/"+id, nil)
	if err != nil {
		return InsightResponse{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return InsightResponse{}, err
	}
	defer resp.Body.Close()

	return decodeJSONResponse[InsightResponse](resp)
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
		return InsightStreamEvent{Type: typeName, Message: payload}, true, nil
	}

	typeName := strings.ToLower(firstString(
		strings.TrimSpace(eventName),
		lookupString(raw, "event"),
		lookupString(raw, "type"),
		lookupNestedString(raw, "data", "event"),
		lookupNestedString(raw, "data", "type"),
	))

	errorText := firstString(
		lookupString(raw, "error"),
		lookupNestedString(raw, "error", "message"),
	)

	done := typeName == "done" || typeName == "complete" || typeName == "completed" || typeName == "finish" || typeName == "finished" || lookupBool(raw, "done") || lookupBool(raw, "completed")
	if typeName == "error" && errorText == "" {
		errorText = firstString(lookupString(raw, "message"), lookupNestedString(raw, "data", "message"))
	}

	// Extract phase/message for progress-only events (phase_update). Note: we
	// deliberately do NOT pull `content` from agent_thought, tool_output, or
	// other reasoning events - those are the model's thinking, not the final
	// answer. The final answer is returned from the /charts/stream HTTP body.
	phase := firstString(
		lookupString(raw, "phase"),
		lookupNestedString(raw, "data", "phase"),
	)
	message := ""
	if typeName == "phase_update" {
		message = firstRawString(
			lookupRawString(raw, "message"),
			lookupNestedRawString(raw, "data", "message"),
		)
	}

	event := InsightStreamEvent{
		Type:      firstString(typeName, "message"),
		Phase:     phase,
		Message:   message,
		Error:     errorText,
		Done:      done,
		RequestID: firstString(lookupString(raw, "request_id"), lookupNestedString(raw, "data", "request_id"), lookupNestedString(raw, "meta", "request_id")),
		ChatID:    firstString(lookupString(raw, "chat_id"), lookupNestedString(raw, "data", "chat_id"), lookupNestedString(raw, "meta", "chat_id")),
		Raw:       raw,
	}

	if event.Error != "" {
		event.Type = "error"
	}

	return event, true, nil
}

// ExtractTable returns a table parsed from an arbitrary response map if one
// can be found. Exported so app/UI layers can reuse the same detection logic
// against HTTP response bodies.
func ExtractTable(raw map[string]any) *InsightTable {
	return extractTable(raw)
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
	if value == nil {
		return ""
	}
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

// BuildDataRowsTable parses a JSON array of objects and produces an
// InsightTable whose column order matches the JSON key insertion order of
// the first object. Sparse rows contribute additional columns appended in
// the order they are first encountered. Returns nil for empty arrays or
// non-array payloads.
func BuildDataRowsTable(rawDataJSON []byte) *InsightTable {
	rows, err := decodeRowsPreservingOrder(rawDataJSON)
	if err != nil || len(rows) == 0 {
		return nil
	}

	columns := make([]string, 0)
	seen := make(map[string]struct{})
	for _, row := range rows {
		for _, key := range row.keys {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			columns = append(columns, key)
		}
	}
	if len(columns) == 0 {
		return nil
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells := make([]string, len(columns))
		for i, col := range columns {
			if value, ok := row.values[col]; ok {
				cells[i] = formatScalar(value)
			}
		}
		tableRows = append(tableRows, cells)
	}

	return &InsightTable{Columns: columns, Rows: tableRows}
}

// BuildTile parses a JSON array of objects and returns the first scalar
// key/value pair from the first row as a TileData. Returns nil if the
// payload is empty, malformed, or has no scalar keys.
func BuildTile(rawDataJSON []byte) *TileData {
	rows, err := decodeRowsPreservingOrder(rawDataJSON)
	if err != nil || len(rows) == 0 {
		return nil
	}
	first := rows[0]
	if len(first.keys) == 0 {
		return nil
	}
	key := first.keys[0]
	value := first.values[key]
	return &TileData{
		Label: key,
		Value: formatScalar(value),
	}
}

// orderedRow holds a single decoded JSON object, preserving the insertion
// order of its keys.
type orderedRow struct {
	keys   []string
	values map[string]any
}

func decodeRowsPreservingOrder(rawDataJSON []byte) ([]orderedRow, error) {
	trimmed := bytes.TrimSpace(rawDataJSON)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '[' {
		return nil, fmt.Errorf("data is not a JSON array")
	}

	rows := make([]orderedRow, 0)
	for dec.More() {
		row, err := decodeOrderedObject(dec)
		if err != nil {
			return nil, err
		}
		if row != nil {
			rows = append(rows, *row)
		}
	}
	return rows, nil
}

// decodeOrderedObject reads a single JSON object from dec, preserving the
// key order. Non-object array entries are skipped (returns nil, nil).
func decodeOrderedObject(dec *json.Decoder) (*orderedRow, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		// Skip non-object scalar entries.
		return nil, nil
	}
	if delim != '{' {
		// Nested array; skip to its closing bracket.
		if err := skipUntilClose(dec, delim); err != nil {
			return nil, err
		}
		return nil, nil
	}

	row := &orderedRow{
		keys:   make([]string, 0),
		values: make(map[string]any),
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", keyTok)
		}

		var value any
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}

		if _, exists := row.values[key]; !exists {
			row.keys = append(row.keys, key)
		}
		row.values[key] = value
	}

	// Consume closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return row, nil
}

func skipUntilClose(dec *json.Decoder, open json.Delim) error {
	close := json.Delim(']')
	if open == '{' {
		close = json.Delim('}')
	}
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			if d == open {
				depth++
			} else if d == close {
				depth--
			}
		}
	}
	return nil
}

// formatScalar renders a decoded JSON value (after json.Decoder.UseNumber)
// to its display string. Numbers are rendered without exponent for
// integer-valued floats; bools become "true"/"false"; nils render as the
// empty string.
func formatScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case json.Number:
		return v.String()
	case float64:
		// Trim trailing .0 for integer-valued floats.
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	default:
		return fmt.Sprint(v)
	}
}
