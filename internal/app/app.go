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
	"github.com/papermap/papermap-tui/internal/ui/components"
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

// insightHTTPResultMsg carries the final answer body from POST
// /api/v1/analytics/charts/stream. This is the authoritative final response;
// SSE events are progress-only and never carry the final text.
type insightHTTPResultMsg struct {
	requestID string
	response  *api.InsightResponse
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
	phase     string
	status    string
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

// sessionExpiredMsg signals that the stored credentials are no longer valid
// and could not be refreshed. The app clears state and routes the user back
// to the login screen.
type sessionExpiredMsg struct {
	reason string
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
	pendingResponse  *api.InsightResponse
	pendingRequestID string
	httpReceived     bool
	sseComplete      bool
	theme            theme.Theme
	landing          landing.Model
	login            uitauth.Model
	chat             chat.Model
	workspace        workspace.Model
	store            *auth.TokenStore
	spinner          spinner.Model
	confirmQuit      bool
	confirmQuitYes   bool
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
		if m.screen == screenChat {
			updatedChat, cmd := m.chat.Update(msg)
			m.chat = updatedChat
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
		m.resetInsightState()
		m.chat.Clear()
		return m, nil

	case workspaceLoadedMsg:
		if msg.err != "" {
			// Workspace load errors are non-fatal - user can still use chat.
			return m, nil
		}
		m.applyWorkspace(msg.workspace)
		return m, nil

	case chat.SubmitMsg:
		// Clear startup errors once user is actively using chat.
		m.startupErr = nil
		return m, m.startInsight(msg)

	case insightStartPair:
		// Dispatch the SSE startup outcome through the normal handler and
		// kick off the HTTP-result waiter in parallel.
		newModel, startCmd := m.Update(msg.started)
		httpCmd := awaitInsightHTTPResult(msg.httpResult)
		return newModel, tea.Batch(startCmd, httpCmd)

	case insightStartedMsg:
		if msg.err != "" {
			m.chat.FailStream(msg.err)
			return m, nil
		}

		m.stream = msg.stream
		m.fallback = msg.fallback
		m.pendingRequestID = msg.requestID
		m.httpReceived = false
		m.sseComplete = false
		m.pendingResponse = nil
		m.chat.SetStreamingIDs(msg.chatID, msg.requestID)
		if msg.stream == nil {
			// No SSE stream available (opening failed silently upstream, or
			// we received the response synchronously). Fall back to awaiting
			// just the HTTP result.
			m.sseComplete = true
			return m, nil
		}
		return m, m.continueInsightStream()

	case insightHTTPResultMsg:
		if msg.err != "" {
			m.closeStream()
			m.resetInsightState()
			m.chat.FailStream(msg.err)
			return m, nil
		}
		// Ignore stale results from a previous request.
		if msg.requestID != "" && m.pendingRequestID != "" && msg.requestID != m.pendingRequestID {
			return m, nil
		}
		m.pendingResponse = msg.response
		m.httpReceived = true
		return m.tryFinalizeInsight()

	case insightChunkMsg:
		if msg.err != "" {
			m.closeStream()
			m.resetInsightState()
			m.chat.FailStream(msg.err)
			return m, nil
		}

		m.chat.SetStreamingIDs(msg.chatID, msg.requestID)

		// SSE events are progress-only. Surface phase_update messages as a
		// lightweight status line; ignore agent_thought and tool events.
		if strings.TrimSpace(msg.status) != "" {
			m.chat.SetStreamStatus(msg.status)
		}

		if !msg.done {
			return m, m.continueInsightStream()
		}

		// `complete` sentinel received. Close the SSE, then finalize if the
		// HTTP body has also arrived.
		m.closeStream()
		m.sseComplete = true
		return m.tryFinalizeInsight()

	case historyLoadedMsg:
		if msg.err != "" {
			// Don't treat history load errors as fatal - just log and continue.
			// The user can still use chat even if history failed to load.
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

	case sessionExpiredMsg:
		m.closeStream()
		m.resetInsightState()
		m.authenticated = false
		m.user = auth.User{}
		_ = m.store.Clear()
		m.chat.Clear()
		reason := strings.TrimSpace(msg.reason)
		if reason == "" {
			reason = "Your session expired. Please sign in again."
		}
		m.login.Reset()
		m.login.SetError(reason)
		m.screen = screenLogin
		return m, nil

	case tea.KeyPressMsg:
		if m.confirmQuit {
			return m.updateQuitConfirm(msg)
		}

		if msg.String() == keyQuit {
			m.confirmQuit = true
			m.confirmQuitYes = false
			return m, nil
		}

		if m.screen == screenLogin {
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
			m.resetInsightState()
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

	base := m.frame(content)

	if m.confirmQuit {
		base = m.overlayQuitDialog(base)
	}

	v := tea.NewView(base)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (m Model) overlayQuitDialog(base string) string {
	dialog := components.ConfirmDialog{
		Title:       "Are you sure you want to quit?",
		Yes:         "Yep!",
		No:          "Nope",
		YesSelected: m.confirmQuitYes,
	}
	overlay := dialog.View(m.theme, m.width)

	baseW := lipgloss.Width(base)
	baseH := lipgloss.Height(base)
	if baseW <= 0 && m.width > 0 {
		baseW = m.width
	}
	if baseH <= 0 && m.height > 0 {
		baseH = m.height
	}

	ow := lipgloss.Width(overlay)
	oh := lipgloss.Height(overlay)
	x := (baseW - ow) / 2
	y := (baseH - oh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	baseLayer := lipgloss.NewLayer(base).Z(0)
	overlayLayer := lipgloss.NewLayer(overlay).X(x).Y(y).Z(1)

	return lipgloss.NewCompositor(baseLayer, overlayLayer).Render()
}

func (m Model) updateQuitConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "right", "tab", "shift+tab", "h", "l":
		m.confirmQuitYes = !m.confirmQuitYes
		return m, nil
	case "y", "Y":
		return m, tea.Quit
	case "n", "N", keyEscape:
		m.confirmQuit = false
		m.confirmQuitYes = false
		return m, nil
	case keyEnter:
		if m.confirmQuitYes {
			return m, tea.Quit
		}
		m.confirmQuit = false
		m.confirmQuitYes = false
		return m, nil
	case keyQuit:
		// A second ctrl+c force-quits as an escape hatch.
		return m, tea.Quit
	}
	return m, nil
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

		// Wire up the refresher so the token store can refresh access tokens
		// on demand once the session is live.
		m.store.SetRefresher(newRefresher(client, m.store))

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

	// For chat screen, don't center - just return styled content to prevent clipping.
	if m.screen == screenChat {
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
	if m.client == nil {
		return func() tea.Msg {
			return insightStartedMsg{err: "API client is not ready yet."}
		}
	}

	// Resolve chatID (possibly creating a new chat), build the request, then
	// fan out two parallel commands:
	//   1. Fire the HTTP POST /charts/stream and deliver its final response.
	//   2. Subscribe to the SSE progress stream for the same request_id.
	// Both have to share a single request_id, so we do the prep work in the
	// outer closure and return a tea.Batch of both commands.
	return func() tea.Msg {
		chatID := m.chat.ChatID()
		if chatID == "" && strings.TrimSpace(m.defaultDashboard) != "" {
			created, err := m.client.CreateChat(context.Background(), api.ChatCreateRequest{
				DashboardID: m.defaultDashboard,
			})
			if err != nil {
				if expired, ok := sessionExpiredFromError(err); ok {
					return expired
				}
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

		// HTTP call runs in a goroutine so its result can be picked up by a
		// dedicated tea.Cmd below. Buffered so the goroutine never blocks.
		httpResult := make(chan insightHTTPResultMsg, 1)
		client := m.client
		go func() {
			response, err := client.StartInsight(context.Background(), request)
			if err != nil {
				if expired, ok := sessionExpiredFromError(err); ok {
					// Surface session-expiry by setting an error so the UI
					// routes through sessionExpiredFromError at receive time.
					_ = expired
					httpResult <- insightHTTPResultMsg{requestID: requestID, err: err.Error()}
					return
				}
				httpResult <- insightHTTPResultMsg{requestID: requestID, err: err.Error()}
				return
			}
			httpResult <- insightHTTPResultMsg{requestID: requestID, response: &response}
		}()

		// Open the SSE subscription with a small retry loop. This must race
		// with the HTTP call so the request_id is already registered.
		result := startInsightWithRetry(m.client, chatID, requestID)
		started := insightStartedMsg{
			chatID:    result.chatID,
			requestID: result.requestID,
			stream:    result.stream,
		}
		if result.err != nil {
			if expired, ok := sessionExpiredFromError(result.err); ok {
				// Drain the HTTP goroutine so it doesn't leak, then route to
				// session-expired.
				go func() { <-httpResult }()
				return expired
			}
			started.err = result.err.Error()
		}

		// Return a composite message carrying the started event plus a
		// follow-up command that awaits the HTTP body.
		return insightStartPair{
			started:    started,
			httpResult: httpResult,
		}
	}
}

// insightStartPair is an internal message used to hand off both the SSE
// startup outcome and a channel that will deliver the HTTP response once it
// resolves. The Update handler splits this into the two real messages.
type insightStartPair struct {
	started    insightStartedMsg
	httpResult <-chan insightHTTPResultMsg
}

func awaitInsightHTTPResult(ch <-chan insightHTTPResultMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
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
		if err != nil {
			if errors.Is(err, io.EOF) {
				return insightChunkMsg{
					chatID:    event.ChatID,
					requestID: event.RequestID,
					done:      true,
				}
			}
			if expired, ok := sessionExpiredFromError(err); ok {
				return expired
			}
			return insightChunkMsg{err: err.Error()}
		}

		if event.Error != "" {
			return insightChunkMsg{err: event.Error}
		}

		// Translate progress events into a status line. Ignore thinking and
		// tool events entirely - they carry no user-facing final output.
		status := ""
		switch event.Type {
		case "phase_update":
			status = strings.TrimSpace(event.Message)
			if status == "" {
				status = humanizePhase(event.Phase)
			}
		}

		return insightChunkMsg{
			chatID:    event.ChatID,
			requestID: event.RequestID,
			phase:     event.Phase,
			status:    status,
			done:      event.Done,
		}
	}
}

func humanizePhase(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "initializing":
		return "Initializing..."
	case "preparing":
		return "Preparing..."
	case "analyzing":
		return "Analyzing..."
	case "executing":
		return "Running analysis..."
	case "finalizing":
		return "Finalizing..."
	default:
		return ""
	}
}

func (m Model) loadConversationHistory(chatID string, fallback []chat.Message) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return historyLoadedMsg{err: "API client is not ready yet."}
		}

		messages, err := m.client.ConversationHistory(context.Background(), chatID)
		if err != nil {
			if expired, ok := sessionExpiredFromError(err); ok {
				return expired
			}
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

// resetInsightState clears per-request buffers and coordination flags. Call
// on any path that tears down an in-flight insight (error, clear, logout,
// session expiry, completion).
func (m *Model) resetInsightState() {
	m.fallback = nil
	m.pendingResponse = nil
	m.pendingRequestID = ""
	m.httpReceived = false
	m.sseComplete = false
}

// tryFinalizeInsight renders the final assistant message once BOTH the HTTP
// body (the only source of the final answer) and the SSE `complete` sentinel
// have arrived. If either is still pending, it returns without rendering.
func (m Model) tryFinalizeInsight() (tea.Model, tea.Cmd) {
	if !m.httpReceived || !m.sseComplete {
		return m, nil
	}

	response := m.pendingResponse
	fallback := m.fallback
	chatID := strings.TrimSpace(m.chat.ChatID())
	m.resetInsightState()
	m.chat.SetStreamStatus("")

	if response != nil {
		content := strings.TrimSpace(response.TextResponse)
		if content == "" {
			content = strings.TrimSpace(response.Thoughts)
		}
		table := toChatTable(extractTableFromResponse(response))
		if content != "" || table != nil {
			m.chat.ReplaceLastAssistant([]chat.Message{{
				Role:    "alan",
				Content: content,
				Table:   table,
			}})
		} else {
			m.chat.CompleteStream()
		}
	} else {
		m.chat.CompleteStream()
	}

	// Conversation history refresh still provides the authoritative record
	// of the exchange (e.g. chart thumbnails, server-side post-processing).
	if chatID != "" {
		return m, m.loadConversationHistory(chatID, fallback)
	}
	if len(fallback) > 0 {
		m.chat.ReplaceLastAssistant(fallback)
	}
	return m, nil
}

// extractTableFromResponse pulls a table out of the HTTP response payload if
// one is present in the raw JSON. The Data field on InsightResponse is a free
// `any` whose shape varies, so we delegate to the existing extractor over the
// raw map.
func extractTableFromResponse(response *api.InsightResponse) *api.InsightTable {
	if response == nil || response.Raw == nil {
		return nil
	}
	return extractTableRaw(response.Raw)
}

// extractTableRaw is a thin local alias around api.ExtractTable so callers in
// this file read consistently.
func extractTableRaw(raw map[string]any) *api.InsightTable {
	return api.ExtractTable(raw)
}

// sessionExpiredFromError checks if err is an auth.ErrSessionExpired and, if
// so, returns a sessionExpiredMsg. The second return value indicates whether
// it matched. Callers use this to route token-expiry errors through the app's
// session-expired flow rather than surfacing them as generic failures.
func sessionExpiredFromError(err error) (tea.Msg, bool) {
	if err == nil {
		return nil, false
	}
	if errors.Is(err, auth.ErrSessionExpired) {
		return sessionExpiredMsg{reason: "Your session expired. Please sign in again."}, true
	}
	// The api client wraps bearer-load errors. Match on the sentinel by
	// string as a fallback since some error chains unwrap through fmt.Errorf
	// with %v rather than %w.
	if strings.Contains(err.Error(), auth.ErrSessionExpired.Error()) {
		return sessionExpiredMsg{reason: "Your session expired. Please sign in again."}, true
	}
	return nil, false
}
