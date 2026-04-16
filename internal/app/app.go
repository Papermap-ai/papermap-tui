package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/config"
	"github.com/papermap/papermap-tui/internal/theme"
	uitauth "github.com/papermap/papermap-tui/internal/ui/auth"
	"github.com/papermap/papermap-tui/internal/ui/chat"
	"github.com/papermap/papermap-tui/internal/ui/landing"
	"github.com/papermap/papermap-tui/internal/ui/workspace"
)

type screen string

const (
	screenSplash          screen = "splash"
	screenLanding         screen = "landing"
	screenLogin           screen = "login"
	screenChat            screen = "chat"
	screenWorkspacePicker screen = "workspace_picker"
)

type startupMsg struct {
	config        config.Config
	authenticated bool
	client        *api.Client
	workspace     *api.UnifiedWorkspace
	err           error
}

type loginResultMsg struct {
	workspace *api.UnifiedWorkspace
	err       string
}

type workspaceLoadedMsg struct {
	workspace *api.UnifiedWorkspace
	err       string
}

type insightStartedMsg struct {
	chatID    string
	requestID string
	stream    *api.InsightStream
	response  *api.InsightResponse
	fallback  []chat.Message
	err       string
}

type startInsightResult struct {
	chatID    string
	requestID string
	stream    *api.InsightStream
	err       error
}

type insightChunkMsg struct {
	chatID    string
	requestID string
	text      string
	table     *chat.Table
	fallback  []chat.Message
	done      bool
	err       string
}

type historyLoadedMsg struct {
	chatID   string
	messages []chat.Message
	fallback []chat.Message
	err      string
}

type Model struct {
	width            int
	height           int
	screen           screen
	config           config.Config
	authenticated    bool
	workspaceName    string
	workspaceID      string
	defaultDashboard string
	user             auth.User
	startupErr       error
	client           *api.Client
	stream           *api.InsightStream
	fallback         []chat.Message
	theme            theme.Theme
	landing          landing.Model
	login            uitauth.Model
	chat             chat.Model
	workspace        workspace.Model
	store            *auth.TokenStore
	spinner          spinner.Model
}

func Run() error {
	model, err := NewModel()
	if err != nil {
		return err
	}

	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}

func NewModel() (Model, error) {
	store, err := auth.DefaultStore()
	if err != nil {
		return Model{}, err
	}

	th := theme.Default()

	return Model{
		screen:        screenSplash,
		workspaceName: "Unified Workspace",
		theme:         th,
		landing:       landing.NewModel(),
		login:         uitauth.NewModel(),
		chat:          chat.NewModel(th),
		workspace:     workspace.NewModel(),
		store:         store,
		spinner:       newSplashSpinner(th),
	}, nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadStartup(), m.spinner.Tick, m.chat.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startupMsg:
		m.config = msg.config
		m.authenticated = msg.authenticated
		m.client = msg.client
		m.startupErr = msg.err
		m.applyWorkspace(msg.workspace)
		if m.authenticated {
			if cred, err := m.store.Load(); err == nil {
				m.user = cred.User
			}
			m.screen = screenChat
		} else {
			m.screen = screenLanding
		}
		return m, nil

	case spinner.TickMsg:
		if m.screen == screenSplash {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		updatedChat, cmd := m.chat.Update(msg)
		m.chat = updatedChat
		return m, cmd

	case uitauth.SubmitMsg:
		m.login.SetSubmitting(true)
		return m, m.initiateLogin(msg)

	case loginResultMsg:
		if msg.err != "" {
			m.login.SetError(msg.err)
			return m, nil
		}

		m.login.Reset()
		m.authenticated = true
		if cred, err := m.store.Load(); err == nil {
			m.user = cred.User
		}
		m.applyWorkspace(msg.workspace)
		m.screen = screenChat
		m.closeStream()
		m.fallback = nil
		m.chat.Clear()
		return m, nil

	case workspaceLoadedMsg:
		if msg.err != "" {
			m.startupErr = errors.New(msg.err)
			return m, nil
		}
		m.applyWorkspace(msg.workspace)
		return m, nil

	case chat.SubmitMsg:
		return m, m.startInsight(msg)

	case insightStartedMsg:
		if msg.err != "" {
			m.chat.FailStream(msg.err)
			return m, nil
		}

		m.stream = msg.stream
		m.fallback = msg.fallback
		m.chat.SetStreamingIDs(msg.chatID, msg.requestID)
		if msg.stream == nil {
			if msg.response != nil {
				m.chat.CompleteStream()
				if len(msg.fallback) > 0 {
					m.chat.ReplaceLastAssistant(msg.fallback)
				}
			}
			m.fallback = nil
			return m, nil
		}
		return m, m.continueInsightStream()

	case insightChunkMsg:
		if msg.err != "" {
			m.closeStream()
			m.chat.FailStream(msg.err)
			return m, nil
		}

		m.chat.SetStreamingIDs(msg.chatID, msg.requestID)
		if msg.table != nil {
			m.chat.SetStreamTable(msg.table)
		}
		if msg.text != "" {
			m.chat.AppendStreamText(msg.text)
		}
		if !msg.done {
			return m, m.continueInsightStream()
		}

		m.closeStream()
		m.chat.CompleteStream()
		if chatID := m.chat.ChatID(); chatID != "" {
			return m, m.loadConversationHistory(chatID, msg.fallback)
		}
		if len(msg.fallback) > 0 {
			m.chat.ReplaceLastAssistant(msg.fallback)
		}
		m.fallback = nil
		return m, nil

	case historyLoadedMsg:
		if msg.err != "" {
			m.startupErr = errors.New(msg.err)
			return m, nil
		}
		if len(msg.messages) == 0 && len(msg.fallback) > 0 {
			m.chat.ReplaceLastAssistant(msg.fallback)
			m.fallback = nil
			return m, nil
		}
		if len(msg.messages) == 0 {
			return m, nil
		}
		m.chat.ReplaceHistory(msg.messages)
		m.fallback = nil
		return m, nil

	case tea.KeyPressMsg:
		if m.screen == screenLogin {
			if msg.String() == keyQuit {
				return m, tea.Quit
			}
			if msg.String() == keyEscape {
				return m.handleEscape(), nil
			}

			updatedLogin, cmd := m.login.Update(msg)
			m.login = updatedLogin
			return m, cmd
		}

		if m.screen == screenChat {
			updatedChat, cmd := m.chat.Update(msg)
			m.chat = updatedChat
			if cmd != nil {
				return m, cmd
			}
		}

		switch msg.String() {
		case keyQuit:
			return m, tea.Quit
		case keyEscape:
			return m.handleEscape(), nil
		case keyEnter:
			return m.handleEnter(), nil
		case keySwitchWorkspace:
			if m.authenticated && m.screen == screenChat {
				m.screen = screenWorkspacePicker
			}
			return m, nil
		case keyClearChat:
			m.closeStream()
			m.fallback = nil
			m.chat.Clear()
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	content := m.viewScreen()
	if m.startupErr != nil {
		content = strings.Join([]string{
			m.theme.Error.Render("Startup error: " + m.startupErr.Error()),
			"",
			content,
		}, "\n")
	}

	return tea.NewView(m.frame(content))
}

func (m Model) loadStartup() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return startupMsg{err: err}
		}

		client, err := api.NewClient(cfg.APIURL, nil, m.store)
		if err != nil {
			return startupMsg{config: cfg, err: err}
		}

		authenticated, err := m.restoreSession(context.Background(), client)
		if err != nil {
			return startupMsg{config: cfg, client: client, err: err}
		}

		workspace, err := loadUnifiedWorkspaceContext(context.Background(), client)
		if err != nil && authenticated {
			return startupMsg{config: cfg, authenticated: authenticated, client: client, err: err}
		}

		return startupMsg{config: cfg, authenticated: authenticated, client: client, workspace: workspace}
	}
}

func (m Model) restoreSession(ctx context.Context, client *api.Client) (bool, error) {
	cred, err := m.store.Load()
	switch {
	case err == nil:
		if cred.Valid() {
			return true, nil
		}

		if strings.TrimSpace(cred.RefreshToken) == "" {
			if err := m.store.Clear(); err != nil {
				return false, err
			}
			return false, nil
		}

		refreshed, err := client.Refresh(ctx, cred.RefreshToken)
		if err != nil {
			if clearErr := m.store.Clear(); clearErr != nil {
				return false, clearErr
			}
			return false, nil
		}

		updatedCred, err := refreshed.ToCredentials(cred)
		if err != nil {
			return false, err
		}

		if err := m.store.Save(updatedCred); err != nil {
			return false, err
		}

		return true, nil

	case errors.Is(err, os.ErrNotExist):
		return false, nil

	default:
		return false, err
	}
}

func (m Model) handleEscape() Model {
	switch m.screen {
	case screenLogin, screenWorkspacePicker:
		if m.authenticated {
			m.screen = screenChat
		} else {
			m.screen = screenLanding
		}
	case screenChat:
		m.screen = screenLanding
	}

	return m
}

func (m Model) handleEnter() Model {
	switch m.screen {
	case screenLanding:
		if m.authenticated {
			m.screen = screenChat
		} else {
			m.screen = screenLogin
		}
	case screenWorkspacePicker:
		m.screen = screenChat
	}

	return m
}

func (m Model) viewScreen() string {
	switch m.screen {
	case screenSplash:
		return m.splashView()
	case screenLogin:
		return m.login.View(m.theme, m.width)
	case screenChat:
		return m.chat.View(m.theme, m.workspaceName, m.width)
	case screenWorkspacePicker:
		return m.workspace.View(m.theme, m.width)
	default:
		return m.landing.View(m.theme, m.width)
	}
}

func (m Model) frame(content string) string {
	styled := m.theme.App.Render(content)
	if m.width <= 0 || m.height <= 0 {
		return styled
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styled)
}

func (m Model) initiateLogin(msg uitauth.SubmitMsg) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return loginResultMsg{err: "API client is not ready yet."}
		}

		result, err := m.client.Login(context.Background(), msg.Email, msg.Password)
		if err != nil {
			return loginResultMsg{err: err.Error()}
		}

		cred, err := result.ToCredentials(auth.Credentials{})
		if err != nil {
			return loginResultMsg{err: err.Error()}
		}

		if err := m.store.Save(cred); err != nil {
			return loginResultMsg{err: err.Error()}
		}

		workspace, err := loadUnifiedWorkspaceContext(context.Background(), m.client)
		if err != nil {
			return loginResultMsg{err: err.Error()}
		}

		return loginResultMsg{workspace: workspace}
	}
}

func (m Model) startInsight(msg chat.SubmitMsg) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return insightStartedMsg{err: "API client is not ready yet."}
		}

		chatID := m.chat.ChatID()
		if chatID == "" && strings.TrimSpace(m.defaultDashboard) != "" {
			created, err := m.client.CreateChat(context.Background(), api.ChatCreateRequest{
				DashboardID: m.defaultDashboard,
			})
			if err != nil {
				return insightStartedMsg{err: err.Error()}
			}
			chatID = strings.TrimSpace(created.LLMDataChatID)
		}

		requestID := api.GenerateRequestID()
		useSearch := true
		request := api.InsightRequest{
			Prompt:                  msg.Prompt,
			WorkspaceID:             m.workspaceID,
			ChatID:                  chatID,
			RequestID:               requestID,
			UseSearch:               &useSearch,
			AllowWorkspaceSwitching: true,
			StreamExecution:         true,
			DisplayPrompt:           msg.Prompt,
			InteractionSource:       "user_query",
		}

		resultCh := make(chan startInsightResult, 1)

		go func() {
			_, _ = m.client.StartInsight(context.Background(), request)
		}()

		go func() {
			resultCh <- startInsightWithRetry(m.client, chatID, requestID)
		}()

		result := <-resultCh
		if result.err != nil {
			return insightStartedMsg{
				chatID:    result.chatID,
				requestID: result.requestID,
				err:       result.err.Error(),
			}
		}

		return insightStartedMsg{
			chatID:    result.chatID,
			requestID: result.requestID,
			stream:    result.stream,
		}
	}
}

func startInsightWithRetry(client *api.Client, chatID string, requestID string) startInsightResult {
	time.Sleep(150 * time.Millisecond)

	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		stream, err := client.OpenInsightStream(context.Background(), api.InsightStreamRequest{RequestID: requestID})
		if err != nil {
			lastErr = err
			time.Sleep(150 * time.Millisecond)
			continue
		}

		firstEvent, err := stream.Next()
		if err == nil && !shouldRetryStreamRace(firstEvent, nil) && firstEvent.Error == "" {
			return startInsightResult{
				chatID:    firstNonEmpty(firstEvent.ChatID, chatID),
				requestID: firstNonEmpty(firstEvent.RequestID, requestID),
				stream:    api.PrependInsightStream(stream, firstEvent),
			}
		}

		_ = stream.Close()
		if shouldRetryStreamRace(firstEvent, err) {
			lastErr = raceError(firstEvent, err)
			time.Sleep(150 * time.Millisecond)
			continue
		}

		if err != nil {
			lastErr = err
		} else if firstEvent.Error != "" {
			lastErr = errors.New(firstEvent.Error)
		}
		break
	}

	if lastErr == nil {
		lastErr = errors.New("stream did not become ready")
	}

	return startInsightResult{chatID: chatID, requestID: requestID, err: lastErr}
}

func shouldRetryStreamRace(event api.InsightStreamEvent, err error) bool {
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(event.Error))
	if message == "" {
		message = strings.ToLower(strings.TrimSpace(firstStringFromMap(event.Raw, "message")))
	}

	return strings.Contains(message, "request not found")
}

func raceError(event api.InsightStreamEvent, err error) error {
	if err != nil {
		return err
	}
	if event.Error != "" {
		return errors.New(event.Error)
	}
	if message := firstStringFromMap(event.Raw, "message"); strings.TrimSpace(message) != "" {
		return errors.New(message)
	}
	return errors.New("request not found")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstStringFromMap(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, _ := raw[key].(string)
	return value
}

func (m Model) continueInsightStream() tea.Cmd {
	return func() tea.Msg {
		if m.stream == nil {
			return insightChunkMsg{done: true}
		}

		event, err := m.stream.Next()
		table := toChatTable(event.Table)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return insightChunkMsg{
					chatID:    event.ChatID,
					requestID: event.RequestID,
					table:     table,
					done:      true,
				}
			}
			return insightChunkMsg{err: err.Error()}
		}

		if event.Error != "" {
			return insightChunkMsg{err: event.Error}
		}

		return insightChunkMsg{
			chatID:    event.ChatID,
			requestID: event.RequestID,
			text:      event.Text,
			table:     table,
			done:      event.Done,
		}
	}
}

func (m Model) loadConversationHistory(chatID string, fallback []chat.Message) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return historyLoadedMsg{err: "API client is not ready yet."}
		}

		messages, err := m.client.ConversationHistory(context.Background(), chatID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "404") || strings.Contains(strings.ToLower(err.Error()), "not found") {
				return historyLoadedMsg{chatID: chatID, messages: nil, fallback: fallback}
			}
			return historyLoadedMsg{chatID: chatID, err: err.Error()}
		}

		converted := make([]chat.Message, 0, len(messages))
		for _, message := range messages {
			converted = append(converted, chat.Message{
				Role:    normalizeRole(message.Role),
				Content: message.Content,
				Table:   toChatTable(message.Table),
			})
		}

		return historyLoadedMsg{chatID: chatID, messages: converted, fallback: fallback}
	}
}

func toChatTable(table *api.InsightTable) *chat.Table {
	if table == nil {
		return nil
	}

	return &chat.Table{
		Columns: append([]string(nil), table.Columns...),
		Rows:    cloneRows(table.Rows),
	}
}

func cloneRows(rows [][]string) [][]string {
	cloned := make([][]string, 0, len(rows))
	for _, row := range rows {
		cloned = append(cloned, append([]string(nil), row...))
	}
	return cloned
}

func normalizeRole(role string) string {
	trimmed := strings.ToLower(strings.TrimSpace(role))
	switch trimmed {
	case "user", "you":
		return "you"
	default:
		return "alan"
	}
}

func convertInsightMessages(messages []api.InsightMessage) []chat.Message {
	converted := make([]chat.Message, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, chat.Message{
			Role:    normalizeRole(message.Role),
			Content: message.Content,
			Table:   toChatTable(message.Table),
		})
	}
	return converted
}

func loadUnifiedWorkspaceContext(ctx context.Context, client *api.Client) (*api.UnifiedWorkspace, error) {
	workspace, err := client.UnifiedWorkspace(ctx)
	if err != nil || workspace == nil {
		return workspace, err
	}

	_, settings, err := client.IncludedWorkspaces(ctx, workspace.WorkspaceID)
	if err == nil {
		workspace.IncludedWorkspaceIDs = append([]string(nil), settings.IncludedWorkspaceIDs...)
	}

	return workspace, nil
}

func (m *Model) applyWorkspace(workspace *api.UnifiedWorkspace) {
	if workspace == nil {
		return
	}

	m.workspaceID = strings.TrimSpace(workspace.WorkspaceID)
	m.defaultDashboard = strings.TrimSpace(workspace.DefaultDashboard)
	if name := strings.TrimSpace(workspace.Name); name != "" {
		m.workspaceName = name
	}
}

func (m *Model) closeStream() {
	if m.stream == nil {
		return
	}
	_ = m.stream.Close()
	m.stream = nil
}
