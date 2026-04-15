package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type UnifiedWorkspace struct {
	WorkspaceID          string   `json:"workspace_id"`
	Name                 string   `json:"name"`
	WorkspaceType        string   `json:"workspace_type"`
	IsUnified            bool     `json:"is_unified"`
	DefaultDashboard     string   `json:"default_dashboard"`
	CreatedAt            string   `json:"created_at,omitempty"`
	IncludedWorkspaceIDs []string `json:"included_workspace_ids,omitempty"`
}

type unifiedWorkspaceEnvelope struct {
	Success   bool              `json:"success"`
	Message   string            `json:"message"`
	Workspace *UnifiedWorkspace `json:"workspace"`
}

type IncludedWorkspace struct {
	WorkspaceID   string `json:"workspace_id"`
	Name          string `json:"name"`
	WorkspaceType string `json:"workspace_type"`
	Included      bool   `json:"included"`
}

type IncludedWorkspaceSettings struct {
	WorkspaceID           string   `json:"workspace_id"`
	WorkspaceName         string   `json:"workspace_name"`
	IncludedWorkspaceIDs  []string `json:"included_workspace_ids"`
	AllWorkspacesIncluded bool     `json:"all_workspaces_included"`
	UpdatedAt             string   `json:"updated_at,omitempty"`
	UpdatedByUserID       string   `json:"updated_by_user_id,omitempty"`
}

type includedWorkspacesEnvelope struct {
	Success    bool                      `json:"success"`
	Message    string                    `json:"message"`
	Workspaces []IncludedWorkspace       `json:"workspaces"`
	Settings   IncludedWorkspaceSettings `json:"settings"`
}

type ChatCreateRequest struct {
	DashboardID string `json:"dashboard_id,omitempty"`
	ReportID    string `json:"report_id,omitempty"`
}

type ChatCreateResponse struct {
	LLMDataChatID string `json:"llm_data_chat_id"`
	DashboardID   string `json:"dashboard_id,omitempty"`
	ReportID      string `json:"report_id,omitempty"`
}

func (c *Client) UnifiedWorkspace(ctx context.Context) (*UnifiedWorkspace, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/analytics/workspaces/unified", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	decoded, err := decodeJSONResponse[unifiedWorkspaceEnvelope](resp)
	if err != nil {
		return nil, err
	}

	if decoded.Workspace == nil {
		return nil, nil
	}

	return decoded.Workspace, nil
}

func (c *Client) IncludedWorkspaces(ctx context.Context, workspaceID string) ([]IncludedWorkspace, IncludedWorkspaceSettings, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, IncludedWorkspaceSettings{}, fmt.Errorf("workspace id is required")
	}

	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/analytics/workspaces/"+workspaceID+"/included-workspaces", nil)
	if err != nil {
		return nil, IncludedWorkspaceSettings{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, IncludedWorkspaceSettings{}, err
	}
	defer resp.Body.Close()

	decoded, err := decodeJSONResponse[includedWorkspacesEnvelope](resp)
	if err != nil {
		return nil, IncludedWorkspaceSettings{}, err
	}

	return decoded.Workspaces, decoded.Settings, nil
}

func (c *Client) CreateChat(ctx context.Context, reqBody ChatCreateRequest) (*ChatCreateResponse, error) {
	if strings.TrimSpace(reqBody.DashboardID) == "" && strings.TrimSpace(reqBody.ReportID) == "" {
		return nil, fmt.Errorf("dashboard id or report id is required")
	}

	req, err := c.NewRequest(ctx, http.MethodPost, "/api/v1/analytics/chats", reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	decoded, err := decodeJSONResponse[ChatCreateResponse](resp)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(decoded.LLMDataChatID) == "" {
		return nil, fmt.Errorf("chat create response missing chat id")
	}

	return &decoded, nil
}
