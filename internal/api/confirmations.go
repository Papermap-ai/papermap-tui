package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// SubmitConfirmationRequest carries the user's allow/deny decision for a
// pending tool-call confirmation surfaced via a `confirmation_required`
// SSE event. The backend agent loop blocks until this endpoint receives a
// matching ConfirmationID for the active RequestID.
type SubmitConfirmationRequest struct {
	RequestID      string `json:"request_id"`
	ConfirmationID string `json:"confirmation_id"`
	Confirmed      bool   `json:"confirmed"`
	// Message is an optional free-text reason. Currently unused by the
	// TUI but accepted by the backend for telemetry.
	Message string `json:"message,omitempty"`
}

// SubmitConfirmationResponse is the unwrapped data payload returned by
// /api/v1/analytics/requests/confirm.
type SubmitConfirmationResponse struct {
	RequestID      string `json:"request_id"`
	ConfirmationID string `json:"confirmation_id"`
	Confirmed      bool   `json:"confirmed"`
}

// SubmitConfirmation forwards the user's allow/deny decision to the
// backend so the blocked agent loop can resume. RequestID and
// ConfirmationID are both required and must match the values carried on
// the originating `confirmation_required` event.
func (c *Client) SubmitConfirmation(ctx context.Context, reqBody SubmitConfirmationRequest) (SubmitConfirmationResponse, error) {
	if strings.TrimSpace(reqBody.RequestID) == "" {
		return SubmitConfirmationResponse{}, fmt.Errorf("request_id is required")
	}
	if strings.TrimSpace(reqBody.ConfirmationID) == "" {
		return SubmitConfirmationResponse{}, fmt.Errorf("confirmation_id is required")
	}

	req, err := c.NewRequest(ctx, http.MethodPost, "/api/v1/analytics/requests/confirm", reqBody)
	if err != nil {
		return SubmitConfirmationResponse{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return SubmitConfirmationResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[SubmitConfirmationResponse](resp)
}
