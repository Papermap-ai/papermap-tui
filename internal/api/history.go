package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ChatHistoryEntry struct {
	LLMDataChatID  string `json:"llm_data_chat_id"`
	Name           string `json:"name"`
	DashboardID    string `json:"dashboard_id,omitempty"`
	ReportID       string `json:"report_id,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	ModifiedAt     string `json:"modified_at,omitempty"`
	Pin            bool   `json:"pin,omitempty"`
	Status         string `json:"status,omitempty"`
	LatestUserName string `json:"latest_user_name,omitempty"`
}

type ChatHistoryPage struct {
	Chats        []ChatHistoryEntry `json:"chats"`
	TotalRecords int                `json:"total_records"`
	TotalPages   int                `json:"total_pages"`
}

type ConversationEntry struct {
	LLMDataID        string `json:"llm_data_id"`
	UserQuery        string `json:"user_query,omitempty"`
	TextResponse     string `json:"text_response"`
	Thoughts         string `json:"thoughts,omitempty"`
	Code             string `json:"code,omitempty"`
	Bookmarked       bool   `json:"bookmarked,omitempty"`
	BookmarkID       string `json:"bookmark_id,omitempty"`
	IsInherited      bool   `json:"is_inherited,omitempty"`
	IsRundown        bool   `json:"is_rundown,omitempty"`
	RundownTurnLabel string `json:"rundown_turn_label,omitempty"`
}

type ConversationsPage struct {
	Conversations  []ConversationEntry `json:"conversations"`
	TotalRecords   int                 `json:"total_records"`
	TotalPages     int                 `json:"total_pages"`
	BranchParentID string              `json:"branch_parent_id,omitempty"`
}

func (c *Client) ListChatHistory(ctx context.Context, dashboardID string, page, perPage int, excludeInsights bool) (ChatHistoryPage, error) {
	dashboardID = strings.TrimSpace(dashboardID)
	if dashboardID == "" {
		return ChatHistoryPage{}, fmt.Errorf("dashboard id is required")
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/analytics/chats-history", nil)
	if err != nil {
		return ChatHistoryPage{}, err
	}

	query := url.Values{}
	query.Set("dashboard_id", dashboardID)
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))
	query.Set("exclude_insights", strconv.FormatBool(excludeInsights))
	req.URL.RawQuery = query.Encode()

	resp, err := c.Do(req)
	if err != nil {
		return ChatHistoryPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[ChatHistoryPage](resp)
}

func (c *Client) ListConversations(ctx context.Context, chatID string, page, perPage int) (ConversationsPage, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return ConversationsPage{}, fmt.Errorf("chat id is required")
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 5
	}

	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/analytics/chats/"+chatID+"/conversations", nil)
	if err != nil {
		return ConversationsPage{}, err
	}

	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))
	req.URL.RawQuery = query.Encode()

	resp, err := c.Do(req)
	if err != nil {
		return ConversationsPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	return decodeJSONResponse[ConversationsPage](resp)
}
