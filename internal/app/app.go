package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/api"
	"github.com/papermap/papermap-tui/internal/auth"
	"github.com/papermap/papermap-tui/internal/config"
	"github.com/papermap/papermap-tui/internal/theme"
	"github.com/papermap/papermap-tui/internal/ui/chat"
	"github.com/papermap/papermap-tui/internal/ui/chat/conversations"
	"github.com/papermap/papermap-tui/internal/ui/chat/modelpicker"
	"github.com/papermap/papermap-tui/internal/ui/components/charts"
	"github.com/papermap/papermap-tui/internal/ui/components/dialog"
	"github.com/papermap/papermap-tui/internal/ui/components/palette"
	"github.com/papermap/papermap-tui/internal/ui/landing"
	"github.com/papermap/papermap-tui/internal/ui/workspace"
)

type screen string

const (
	screenSplash          screen = "splash"
	screenLanding         screen = "landing"
	screenChat            screen = "chat"
	screenWorkspacePicker screen = "workspace_picker"
	screenModelPicker     screen = "model_picker"
	screenCommandPalette  screen = "command_palette"
	screenConversations   screen = "conversations"
)

// Minimum terminal dimensions required for the UI to render correctly.
// Below these, we show a notice asking the user to resize. The width is
// chosen to accommodate the landing panel (62 cols) and chat layout; the
// height leaves room for the logo, panel, and key hints.
const (
	minTerminalWidth  = 60
	minTerminalHeight = 20
)

type startupMsg struct {
	config        config.Config
	authenticated bool
	client        *api.Client
	workspace     *api.UnifiedWorkspace
	// models carries the available LLM choices for the current user.
	// May be nil when the fetch failed; the app falls back to a single
	// synthetic choice based on the persisted slug or the hardcoded
	// FallbackDefaultModel so the UI still renders.
	models   []api.ModelChoice
	defModel string
	err      error
}

type workspaceLoadedMsg struct {
	workspace *api.UnifiedWorkspace
	err       string
}

type insightStartedMsg struct {
	chatID    string
	requestID string
	stream    *api.InsightStream
	err       string
}

// insightHTTPResultMsg carries the final answer body from POST
// /api/v1/analytics/charts/stream. This is the authoritative final response;
// SSE events are progress-only and never carry the final text.
type insightHTTPResultMsg struct {
	requestID string
	response  *api.InsightResponse
	err       string
	// sessionExpired indicates the HTTP error was an auth.ErrSessionExpired
	// so the Update handler can route to sessionExpiredMsg instead of
	// surfacing the raw error.
	sessionExpired bool
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
	status    string
	done      bool
	err       string
	// trace carries an optional reasoning/tool event to forward to the
	// chat model. Only one of the trace* fields is set per message.
	trace *insightTraceDispatch
}

// insightTraceDispatch is the union of trace updates produced by the SSE
// stream. The Update handler routes each variant to the matching chat
// helper.
type insightTraceDispatch struct {
	// kind selects the variant: "thought", "step", "output", "complete".
	kind string

	// thought variant.
	iteration       int
	thoughtDelta    string
	thoughtComplete bool

	// step variant (newly created TraceStep, e.g. tool_call_announced /
	// tool_call_args_complete).
	step chat.TraceStep

	// output / complete variants share these fields.
	toolCallID  string
	toolContent string
	toolStatus  string
	toolMS      float64
}

// sessionExpiredMsg signals that the stored credentials are no longer valid
// and could not be refreshed. The app clears state and routes the user back
// to the login screen.
type sessionExpiredMsg struct {
	reason string
}

// insightCancelledMsg signals that a user-initiated cancel has finished
// firing the backend cancel endpoint. The local stream/context teardown
// happens synchronously when esc is handled; this message only carries
// any backend error so the UI can surface a non-blocking notice.
type insightCancelledMsg struct {
	requestID string
	err       string
}

// confirmationRequiredMsg is dispatched by continueInsightStream when
// the SSE stream emits a `confirmation_required` event. It carries the
// fields the modal needs to render and the ids needed to POST back the
// user's decision.
type confirmationRequiredMsg struct {
	requestID         string
	confirmationID    string
	toolDisplayName   string
	message           string
	actionDescription string
	timeoutSeconds    int
}

// confirmationSubmittedMsg carries the result of submitting the user's
// decision. On error the dialog stays open and a transient notice is
// surfaced via cancelNotice; on success the stream is resumed.
type confirmationSubmittedMsg struct {
	requestID      string
	confirmationID string
	confirmed      bool
	err            string
}

type Model struct {
	width            int
	height           int
	screen           screen
	config           config.Config
	configLoaded     bool
	authenticated    bool
	workspaceName    string
	workspaceID      string
	defaultDashboard string
	workspaces       []config.WorkspaceEntry
	workspacesAt     time.Time
	user             auth.User
	startupErr       error
	// landingMessage replaces the default landing copy when set. Used to
	// surface "not signed in" and "session expired" prompts that point
	// users at `papermap auth login`.
	landingMessage   string
	client           *api.Client
	stream           *api.InsightStream
	pendingResponse  *api.InsightResponse
	pendingRequestID string
	httpReceived     bool
	sseComplete      bool
	theme            theme.Theme
	landing          landing.Model
	chat             chat.Model
	workspace        workspace.Model
	modelPicker      modelpicker.Model
	palette          palette.Model
	conversations    conversations.Model
	store            *auth.TokenStore
	spinner          spinner.Model
	// dialog tracks the in-flight confirmation modal (quit confirm or
	// SSE tool-call approval). At most one is active at a time; see
	// internal/app/dialog.go for the controller.
	dialog *pendingDialog
	// insightCancel is the cancel func for the in-flight insight request.
	// It is set when startInsight kicks off and called from cancelInsight
	// on user-initiated teardown (Clear / switch workspace / quit /
	// session expiry). Closing the SSE alone (closeStream) does NOT
	// cancel the HTTP body: that POST is the authoritative source of the
	// final answer and outlives the SSE complete sentinel.
	insightCancel context.CancelFunc
	// cancelNotice surfaces a transient warning after a user-initiated
	// cancel when the backend cancel endpoint failed. The local cancel
	// always succeeds; this only signals that the agent run may keep
	// going server-side. Cleared on the next user action.
	cancelNotice string
	// availableModels is the flattened list of LLM choices fetched at
	// startup. Used by the model picker and TAB cycle.
	availableModels []api.ModelChoice
	// selectedModel is the slug currently routed on outgoing insight
	// requests. Persisted to config.SelectedModel on change.
	selectedModel string
	// defaultModelSlug is the backend-recommended default. Used when
	// the persisted SelectedModel is empty or no longer valid.
	defaultModelSlug string
	// shellCancel is the cancel func for the in-flight "!" shell-mode
	// command. Set in startShellCommand and called from
	// cancelShellCommand on user-initiated teardown (esc / ctrl+l /
	// session expiry / quit). Distinct from insightCancel so a shell
	// command and an insight cannot stomp on each other's contexts.
	shellCancel context.CancelFunc
	// shellPath is the absolute path of the shell binary to invoke
	// for "!" mode. Resolved once at startup from config.Config so
	// PowerShell discovery (Windows) does not run on every command
	// and so a missing pwsh.exe surfaces a clean error before the
	// TUI starts. Empty on platforms that resolve lazily (none today).
	shellPath string
}

type RunOptions struct {
	APIURLOverride string
}

func Run(opts RunOptions) error {
	// Load config and resolve the "!" shell binary up front so an
	// invalid shell.windows value (or a missing pwsh.exe when the
	// user opted into PowerShell) fails before the TUI starts. This
	// is intentional: the chat layer should not surface "shell not
	// found" surprise after the splash screen.
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if v := strings.TrimSpace(opts.APIURLOverride); v != "" {
		cfg.APIURL = v
	}
	shellPath, err := resolveUserShell(cfg)
	if err != nil {
		return err
	}

	model, err := NewModel()
	if err != nil {
		return err
	}
	model.config = cfg
	model.configLoaded = true
	model.shellPath = shellPath

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

	cache, _ := config.LoadWorkspaces()

	return Model{
		screen:        screenSplash,
		workspaceName: "Unified Workspace",
		theme:         th,
		landing:       landing.NewModel(),
		chat:          chat.NewModel(th),
		workspace:     workspace.NewModel(),
		modelPicker:   modelpicker.NewModel(),
		palette:       palette.NewModel(),
		conversations: conversations.NewModel(),
		workspaces:    cache.Workspaces,
		workspacesAt:  cache.UpdatedAt,
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
		return m.handleStartup(msg)
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.MouseWheelMsg, tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		return m.forwardChatIfActive(msg)
	case tea.PasteMsg:
		// Bracketed-paste content is meaningful only inside the chat
		// textarea today. Forward it so the chat model's paste handler
		// can collapse large blobs into chips.
		return m.forwardChatIfActive(msg)
	case workspacesLoadedMsg:
		return m.handleWorkspacesLoaded(msg), nil
	case workspace.SelectMsg:
		return m.switchWorkspace(msg.Workspace), nil
	case workspace.CancelMsg:
		return m.handleWorkspaceCancel(), nil
	case modelpicker.SelectMsg:
		return m.handleModelPickerSelect(msg), nil
	case modelpicker.CancelMsg:
		return m.handleModelPickerCancel(), nil
	case workspaceLoadedMsg:
		return m.handleWorkspaceLoaded(msg), nil
	case chatHistoryLoadedMsg:
		return m.handleChatHistoryLoaded(msg)
	case conversationsLoadedMsg:
		return m.handleConversationsLoaded(msg)
	case chartBackfilledMsg:
		return m.handleChartBackfilled(msg)
	case palette.SelectMsg:
		cmd := m.dispatchPaletteCommand(msg.Command)
		return m, cmd
	case palette.CancelMsg:
		m.screen = screenChat
		return m, nil
	case conversations.OpenChatMsg:
		// Show the conversations panel in a loading state while the
		// page-1 fetch runs so the user sees feedback. The handler
		// flips back to chat on success.
		m.conversations.Reset()
		return m, m.fetchConversations(msg.Chat)
	case conversations.LoadMoreMsg:
		return m, m.fetchChatHistory(m.conversations.Page() + 1)
	case conversations.CancelMsg:
		m.screen = screenChat
		return m, nil
	case chat.SubmitMsg:
		// Clear startup errors once the user is actively chatting.
		m.startupErr = nil
		return m, m.startInsight(msg)
	case chat.ShellSubmitMsg:
		m.startupErr = nil
		return m, m.startShellCommand(msg.Command)
	case shellResultMsg:
		return m.handleShellResult(msg), nil
	case insightStartPair:
		return m.handleInsightStartPair(msg)
	case insightStartedMsg:
		return m.handleInsightStarted(msg)
	case insightHTTPResultMsg:
		return m.handleInsightHTTPResult(msg)
	case insightChunkMsg:
		return m.handleInsightChunk(msg)
	case sessionExpiredMsg:
		return m.handleSessionExpired(msg)
	case insightCancelledMsg:
		return m.handleInsightCancelled(msg)
	case confirmationRequiredMsg:
		return m.handleConfirmationRequired(msg)
	case dialogTickMsg:
		return m.handleDialogTick(msg)
	case confirmationSubmittedMsg:
		return m.handleConfirmationSubmitted(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}
	// Default: forward unknown messages to the chat model when the chat
	// screen is active. This lets chat-side components (toast dismiss
	// ticks, future bubbles) deliver their messages without each one
	// needing an explicit case here. chat.Update is a no-op for any
	// message type it doesn't recognize.
	return m.forwardChatIfActive(msg)
}

func (m Model) handleStartup(msg startupMsg) (tea.Model, tea.Cmd) {
	m.config = msg.config
	m.authenticated = msg.authenticated
	m.client = msg.client
	m.startupErr = msg.err
	m.applyWorkspace(msg.workspace)
	m.hydrateModels(msg.models, msg.defModel)
	if m.authenticated {
		if cred, err := m.store.Load(); err == nil {
			m.user = cred.User
		}
		m.screen = screenChat
		if m.shouldRefreshWorkspaces() {
			return m, loadWorkspacesCmd(m.client)
		}
		return m, nil
	}
	m.landingMessage = "You're not signed in to Papermap."
	m.screen = screenLanding
	return m, nil
}

func (m Model) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenSplash {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	if m.screen == screenChat {
		return m.forwardToChat(msg)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	return m.forwardToChat(msg)
}

// forwardChatIfActive forwards msg to the chat model only when the chat
// screen is the active surface. Used for mouse events that have no meaning
// on other screens.
func (m Model) forwardChatIfActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.screen != screenChat {
		return m, nil
	}
	return m.forwardToChat(msg)
}

// forwardToChat threads msg through chat.Model.Update and stores the
// returned model. Centralizing this avoids the four-line repetition that
// otherwise leaks throughout Update.
func (m Model) forwardToChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedChat, cmd := m.chat.Update(msg)
	m.chat = updatedChat
	return m, cmd
}

func (m Model) handleWorkspacesLoaded(msg workspacesLoadedMsg) Model {
	if msg.err != nil || len(msg.entries) == 0 {
		// Non-fatal. Keep whatever we already have cached. If the picker
		// is open in a loading state, surface an empty result so it stops
		// spinning instead of hanging on "Loading...".
		if m.screen == screenWorkspacePicker && len(m.workspaces) == 0 {
			m.workspace.SetWorkspaces(nil, m.workspaceID)
		}
		return m
	}
	m.workspaces = msg.entries
	m.workspacesAt = time.Now().UTC()
	_ = config.SaveWorkspaces(config.WorkspaceCache{
		Workspaces: msg.entries,
		UpdatedAt:  m.workspacesAt,
	})
	// If the picker is currently open waiting on this fetch, refresh it
	// in place so the loading state clears.
	if m.screen == screenWorkspacePicker {
		m.workspace.SetWorkspaces(m.workspaces, m.workspaceID)
	}
	return m
}

func (m Model) handleWorkspaceCancel() Model {
	if m.authenticated {
		m.screen = screenChat
	} else {
		m.screen = screenLanding
	}
	return m
}

func (m Model) handleWorkspaceLoaded(msg workspaceLoadedMsg) Model {
	if msg.err != "" {
		// Workspace load errors are non-fatal - user can still use chat.
		return m
	}
	m.applyWorkspace(msg.workspace)
	return m
}

func (m Model) handleInsightStartPair(msg insightStartPair) (tea.Model, tea.Cmd) {
	// Stash the cancel func so cancelInsight can tear down the in-flight
	// HTTP/SSE goroutines on user-initiated teardown.
	m.insightCancel = msg.cancel
	newModel, startCmd := m.Update(msg.started)
	httpCmd := awaitInsightHTTPResult(msg.httpResult)
	return newModel, tea.Batch(startCmd, httpCmd)
}

func (m Model) handleInsightStarted(msg insightStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" {
		m.chat.FailStream(msg.err)
		return m, nil
	}
	m.stream = msg.stream
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
}

func (m Model) handleInsightHTTPResult(msg insightHTTPResultMsg) (tea.Model, tea.Cmd) {
	if msg.sessionExpired {
		return m.Update(sessionExpiredMsg{
			reason: "Your session expired. Please sign in again.",
		})
	}
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
}

func (m Model) handleInsightChunk(msg insightChunkMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" {
		m.closeStream()
		m.resetInsightState()
		m.chat.FailStream(msg.err)
		return m, nil
	}
	m.chat.SetStreamingIDs(msg.chatID, msg.requestID)

	// SSE events feed two surfaces: phase_update messages drive a short
	// status line beside the spinner; reasoning and tool lifecycle events
	// feed the per-message trace timeline.
	if strings.TrimSpace(msg.status) != "" {
		m.chat.SetStreamStatus(msg.status)
	}
	if msg.trace != nil {
		m.applyTraceDispatch(msg.trace)
	}

	if !msg.done {
		return m, m.continueInsightStream()
	}

	// `complete` sentinel received. Close the SSE, then finalize if the
	// HTTP body has also arrived.
	m.closeStream()
	m.sseComplete = true
	return m.tryFinalizeInsight()
}

func (m Model) handleSessionExpired(msg sessionExpiredMsg) (tea.Model, tea.Cmd) {
	m.cancelInsight()
	m.resetInsightState()
	m.authenticated = false
	m.user = auth.User{}
	_ = m.store.Clear()
	_ = config.ClearWorkspaces()
	m.clearPersistedModel()
	m.workspaces = nil
	m.workspacesAt = time.Time{}
	m.chat.Clear()
	reason := strings.TrimSpace(msg.reason)
	if reason == "" {
		reason = "Your session expired. Run `papermap auth login` to sign in again."
	}
	m.landingMessage = reason
	m.screen = screenLanding
	return m, tea.Quit
}

// handleInsightCancelled processes the result of the async backend cancel
// call. The local cancel happened synchronously when esc was pressed; this
// only surfaces a transient notice when the backend rejected the cancel
// so the user knows the agent run may continue server-side.
func (m Model) handleInsightCancelled(msg insightCancelledMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" {
		m.cancelNotice = "Cancel may not have reached the server: " + msg.err
	}
	return m, nil
}

// handleConfirmationRequired opens the approval dialog and primes the
// countdown tick loop. Subsequent SSE events are blocked server-side
// until SubmitConfirmation lands, so the chat status line is parked at
// "Awaiting confirmation..." until then.
func (m Model) handleConfirmationRequired(msg confirmationRequiredMsg) (tea.Model, tea.Cmd) {
	requestID := strings.TrimSpace(msg.requestID)
	if requestID == "" {
		requestID = m.pendingRequestID
	}
	confirmationID := strings.TrimSpace(msg.confirmationID)
	if requestID == "" || confirmationID == "" {
		// Malformed event: keep pumping the stream so we don't deadlock.
		return m, m.continueInsightStream()
	}

	correlationID := "conf:" + confirmationID
	req := approvalRequest(msg)
	onResult := m.submitConfirmationCallback(requestID, confirmationID)
	cmd := m.openDialog(correlationID, req, onResult)
	m.chat.SetStreamStatus("Awaiting confirmation...")
	return m, cmd
}

// approvalRequest builds the dialog.Request for an SSE tool-call
// approval. Title/body/detail mirror the pre-refactor ApprovalDialog
// layout: header is the tool display name, body is the human-readable
// prompt from the backend, detail is the verbose action description.
func approvalRequest(msg confirmationRequiredMsg) dialog.Request {
	title := strings.TrimSpace(msg.toolDisplayName)
	if title == "" {
		title = "Tool call approval"
	}
	body := strings.TrimSpace(msg.message)
	if body == "" {
		body = "The agent wants to run a tool. Approve?"
	}
	return dialog.Request{
		Title:  title,
		Body:   body,
		Detail: strings.TrimSpace(msg.actionDescription),
		Actions: []dialog.Action{
			{ID: "deny", Label: "Deny", Tone: dialog.ToneDanger, Hotkey: 'n'},
			{ID: "allow", Label: "Allow", Tone: dialog.ToneAccept, Hotkey: 'y'},
		},
		DefaultID:   "deny",
		DismissID:   "deny",
		TimeoutSecs: msg.timeoutSeconds,
		TimeoutAct:  "deny",
	}
}

// submitConfirmationCallback returns the resolution callback that the
// dialog invokes when the user picks allow or deny (or the timeout
// fires the auto-deny). The callback returns keepOpen=true so the
// modal stays visible during the in-flight POST; on backend success
// handleConfirmationSubmitted clears it, on failure the user can retry.
func (m Model) submitConfirmationCallback(requestID, confirmationID string) func(string) (tea.Cmd, bool) {
	client := m.client
	return func(actionID string) (tea.Cmd, bool) {
		if client == nil {
			return nil, false
		}
		allow := actionID == "allow"
		cmd := func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := client.SubmitConfirmation(ctx, api.SubmitConfirmationRequest{
				RequestID:      requestID,
				ConfirmationID: confirmationID,
				Confirmed:      allow,
			})
			if err != nil {
				return confirmationSubmittedMsg{
					requestID:      requestID,
					confirmationID: confirmationID,
					confirmed:      allow,
					err:            err.Error(),
				}
			}
			return confirmationSubmittedMsg{
				requestID:      requestID,
				confirmationID: confirmationID,
				confirmed:      allow,
			}
		}
		return cmd, true
	}
}

// handleConfirmationSubmitted clears the dialog on success or surfaces
// a transient notice on failure so the user can retry. The retry path
// flips the submitting flag back off so the existing dialog regains
// keyboard focus without rebuilding it.
func (m Model) handleConfirmationSubmitted(msg confirmationSubmittedMsg) (tea.Model, tea.Cmd) {
	correlationID := "conf:" + msg.confirmationID
	if msg.err != "" {
		if m.dialog != nil && m.dialog.correlationID == correlationID {
			m.dialog.submitting = false
		}
		m.cancelNotice = "Could not submit decision: " + msg.err
		return m, nil
	}

	if m.dialog != nil && m.dialog.correlationID == correlationID {
		m.closeDialog()
	}
	m.chat.SetStreamStatus("")
	if m.stream != nil {
		return m, m.continueInsightStream()
	}
	return m, nil
}

// userCancelInsight performs the synchronous teardown of an in-flight
// insight request and returns a tea.Cmd that fires the backend cancel
// endpoint asynchronously. The originating user prompt stays in the
// transcript and the pending assistant slot becomes an inline ERROR
// badge so the cancellation reads as a deliberate event.
func (m *Model) userCancelInsight() tea.Cmd {
	requestID := strings.TrimSpace(m.pendingRequestID)

	m.cancelInsight()
	m.resetInsightState()
	m.cancelNotice = ""
	m.chat.CancelStream()

	if requestID == "" || m.client == nil {
		return nil
	}

	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := client.CancelInsight(ctx, api.CancelInsightRequest{
			RequestID: requestID,
			Reason:    "user_cancelled",
		})
		if err != nil {
			return insightCancelledMsg{requestID: requestID, err: err.Error()}
		}
		return insightCancelledMsg{requestID: requestID}
	}
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C precedence: when a dialog is in flight ctrl+c short-
	// circuits to either dismiss the quit confirm + force quit, or
	// (for any other dialog) raise the quit confirm. The quit
	// dialog's own "y" hotkey path is handled inside updateDialog.
	if msg.String() == keyQuit {
		if m.dialog != nil && m.dialog.correlationID == quitDialogID {
			// Second ctrl+c while quit confirm is up = force quit.
			return m, tea.Quit
		}
		return m, m.openQuitDialog()
	}

	if m.dialog != nil {
		return m.updateDialog(msg)
	}

	if m.screen == screenCommandPalette {
		updated, cmd := m.palette.Update(msg)
		m.palette = updated
		return m, cmd
	}

	if m.screen == screenConversations {
		updated, cmd := m.conversations.Update(msg)
		m.conversations = updated
		return m, cmd
	}

	if m.screen == screenWorkspacePicker {
		updatedPicker, cmd := m.workspace.Update(msg)
		m.workspace = updatedPicker
		return m, cmd
	}

	if m.screen == screenModelPicker {
		updatedPicker, cmd := m.modelPicker.Update(msg)
		m.modelPicker = updatedPicker
		return m, cmd
	}

	if m.screen == screenLanding && !m.authenticated {
		// Unauthenticated landing is a terminal screen; any key quits so
		// the user can run `papermap auth login`.
		return m, tea.Quit
	}

	// Intercept palette and conversations openers BEFORE delegating to
	// chat.Update so the textarea does not consume them. "/" is only an
	// opener when the textarea is empty; otherwise it falls through to
	// be typed as a literal character. ctrl+p is always an opener.
	if m.screen == screenChat && m.authenticated {
		switch msg.String() {
		case keyShellMode:
			// "!" toggles shell mode only when the textarea is
			// empty and no insight or shell command is in flight.
			// Otherwise the keystroke flows through to the
			// textarea so users can still type "!" mid-prompt.
			if m.chat.TextareaIsEmpty() && !m.chat.IsStreaming() && !m.chat.IsShellRunning() && !m.chat.IsShellMode() {
				m.chat.SetShellMode(true)
				return m, nil
			}
		case keyCommandPalette:
			if m.chat.TextareaIsEmpty() && !m.chat.IsStreaming() && !m.chat.IsShellMode() {
				m.openCommandPalette()
				return m, nil
			}
		case keyConversations:
			if !m.chat.IsStreaming() && !m.chat.IsShellRunning() {
				cmd := m.openConversations()
				return m, cmd
			}
		case keyCycleModel:
			// TAB cycles to the next available model. Suppressed
			// while streaming so an in-flight request keeps the
			// model that produced it.
			if !m.chat.IsStreaming() && !m.chat.IsShellMode() {
				return m.cycleModel(), nil
			}
		case keyEscape:
			// Esc precedence (most specific first):
			//   1. cancel an in-flight shell command,
			//   2. exit shell mode (drop any draft),
			//   3. cancel an in-flight insight stream.
			// The chat layer's own selection-clear case still
			// runs first inside chat.Update because that branch
			// is handled before the textarea sees the key.
			if m.chat.IsShellRunning() {
				m.cancelShellCommand()
				return m, nil
			}
			if m.chat.IsShellMode() {
				m.chat.SetShellMode(false)
				return m, nil
			}
			if m.chat.IsStreaming() {
				return m, m.userCancelInsight()
			}
		}
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
			return m, m.openWorkspacePicker()
		}
		return m, nil
	case keyClearChat:
		m.cancelInsight()
		m.cancelShellCommand()
		m.resetInsightState()
		m.chat.CancelShell()
		m.chat.Clear()
		return m, nil
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.terminalTooSmall() {
		v := tea.NewView(m.tooSmallView())
		v.AltScreen = true
		v.MouseMode = tea.MouseModeAllMotion
		return v
	}

	content := m.viewScreen()
	if m.startupErr != nil {
		content = strings.Join([]string{
			m.theme.Error.Render("Startup error: " + m.startupErr.Error()),
			"",
			content,
		}, "\n")
	}
	if m.cancelNotice != "" && m.screen == screenChat {
		content = strings.Join([]string{
			m.theme.Muted.Render(m.cancelNotice),
			"",
			content,
		}, "\n")
	}

	base := m.frame(content)

	if m.screen == screenWorkspacePicker {
		base = m.overlayWorkspacePicker(base)
	}

	if m.screen == screenModelPicker {
		base = m.overlayModelPicker(base)
	}

	if m.screen == screenCommandPalette {
		base = m.overlayCommandPalette(base)
	}

	if m.screen == screenConversations {
		base = m.overlayConversations(base)
	}

	if m.dialog != nil {
		base = m.overlayDialog(base)
	}

	v := tea.NewView(base)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

// quitDialogID is the correlation id used for the "are you sure you
// want to quit?" confirm. Unique per app run; ctrl+c routing keys off
// this id so a second ctrl+c on the quit dialog force-quits.
const quitDialogID = "quit"

// openQuitDialog stages the quit confirmation. Returns the (nil)
// schedule cmd for the dialog so the call site can return it directly
// in the same tea.Cmd slot the previous implementation used.
func (m *Model) openQuitDialog() tea.Cmd {
	req := dialog.Request{
		Title: "Are you sure you want to quit?",
		Actions: []dialog.Action{
			{ID: "no", Label: "Nope", Tone: dialog.ToneNeutral, Hotkey: 'n'},
			{ID: "yes", Label: "Yep!", Tone: dialog.ToneAccept, Hotkey: 'y'},
		},
		DefaultID: "no",
		DismissID: "no",
	}
	onResult := func(actionID string) (tea.Cmd, bool) {
		if actionID == "yes" {
			return m.quitWithCancel(), false
		}
		return nil, false
	}
	return m.openDialog(quitDialogID, req, onResult)
}

// centerOverlay composites overlay over base with the overlay centered both
// horizontally and vertically. Falls back to terminal width/height when the
// rendered base has zero extent (e.g. before the first WindowSizeMsg).
func (m Model) centerOverlay(base, overlay string) string {
	baseW := lipgloss.Width(base)
	baseH := lipgloss.Height(base)
	if baseW <= 0 && m.width > 0 {
		baseW = m.width
	}
	if baseH <= 0 && m.height > 0 {
		baseH = m.height
	}

	x := (baseW - lipgloss.Width(overlay)) / 2
	y := (baseH - lipgloss.Height(overlay)) / 2
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

// quitWithCancel tears down any in-flight insight request and fires a
// best-effort backend cancel before quitting. The backend call is bounded
// by a short timeout so an unresponsive server cannot delay shutdown.
func (m *Model) quitWithCancel() tea.Cmd {
	requestID := strings.TrimSpace(m.pendingRequestID)
	hasClient := m.client != nil
	m.cancelInsight()
	m.resetInsightState()

	if requestID == "" || !hasClient {
		return tea.Quit
	}

	client := m.client
	cancelCmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = client.CancelInsight(ctx, api.CancelInsightRequest{
			RequestID: requestID,
			Reason:    "user_cancelled",
		})
		return nil
	}
	return tea.Sequence(cancelCmd, tea.Quit)
}

func (m Model) loadStartup() tea.Cmd {
	return func() tea.Msg {
		cfg := m.config
		if !m.configLoaded {
			loaded, err := config.Load()
			if err != nil {
				return startupMsg{err: err}
			}
			cfg = loaded
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

		var (
			models   []api.ModelChoice
			defModel string
		)
		if authenticated {
			if opts, optsErr := client.ListModels(context.Background()); optsErr == nil {
				models = opts.Flatten()
				defModel = opts.DefaultSlug()
			}
		}

		return startupMsg{
			config:        cfg,
			authenticated: authenticated,
			client:        client,
			workspace:     workspace,
			models:        models,
			defModel:      defModel,
		}
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

	case errors.Is(err, auth.ErrNoCredentials):
		return false, nil

	default:
		return false, err
	}
}

func (m Model) handleEscape() Model {
	switch m.screen {
	case screenWorkspacePicker, screenModelPicker, screenCommandPalette, screenConversations:
		if m.authenticated {
			m.screen = screenChat
		} else {
			m.screen = screenLanding
		}
	}

	return m
}

func (m Model) handleEnter() Model {
	if m.screen == screenLanding && m.authenticated {
		m.screen = screenChat
	}

	return m
}

func (m Model) viewScreen() string {
	switch m.screen {
	case screenSplash:
		return m.splashView()
	case screenChat, screenWorkspacePicker, screenModelPicker, screenCommandPalette, screenConversations:
		return m.chat.View(m.theme, m.workspaceName, m.width)
	default:
		return m.landing.View(m.theme, m.width, m.landingMessage)
	}
}

func (m Model) frame(content string) string {
	styled := m.theme.App.Render(content)
	if m.width <= 0 || m.height <= 0 {
		return styled
	}

	// For chat-derived screens (chat + overlays), don't center - just
	// return styled content to prevent clipping.
	if m.screen == screenChat ||
		m.screen == screenWorkspacePicker ||
		m.screen == screenModelPicker ||
		m.screen == screenCommandPalette ||
		m.screen == screenConversations {
		return styled
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styled)
}

// terminalTooSmall reports whether the current terminal dimensions are below
// the minimum required for the UI to render correctly. Returns false while
// dimensions are still unknown (before the first WindowSizeMsg).
func (m Model) terminalTooSmall() bool {
	if m.width <= 0 || m.height <= 0 {
		return false
	}
	return m.width < minTerminalWidth || m.height < minTerminalHeight
}

// tooSmallView renders a centered notice asking the user to resize their
// terminal. Shown in place of any screen when the terminal falls below the
// minimum supported size.
func (m Model) tooSmallView() string {
	lines := []string{
		m.theme.Accent.Render("Terminal too small"),
		"",
		m.theme.Body.Render("Papermap needs a larger window to render."),
		m.theme.Muted.Render(fmt.Sprintf(
			"Current: %d×%d   Minimum: %d×%d",
			m.width, m.height, minTerminalWidth, minTerminalHeight,
		)),
		"",
		m.theme.KeyHint.Render("Resize your terminal  •  Ctrl+C quit"),
	}
	content := strings.Join(lines, "\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
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
	// Both have to share a single request_id and a single ctx, so we do the
	// prep work in the outer closure and return a composite message.
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())

		chatID := m.chat.ChatID()
		if chatID == "" && strings.TrimSpace(m.defaultDashboard) != "" {
			created, err := m.client.CreateChat(ctx, api.ChatCreateRequest{
				DashboardID: m.defaultDashboard,
			})
			if err != nil {
				cancel()
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
			LLMModel:                strings.TrimSpace(m.selectedModel),
		}

		// HTTP call runs in a goroutine so its result can be picked up by a
		// dedicated tea.Cmd below. Buffered so the goroutine never blocks
		// even if Update has already torn the request down.
		httpResult := make(chan insightHTTPResultMsg, 1)
		client := m.client
		go func() {
			response, err := client.StartInsight(ctx, request)
			if err != nil {
				if _, ok := sessionExpiredFromError(err); ok {
					httpResult <- insightHTTPResultMsg{
						requestID:      requestID,
						err:            err.Error(),
						sessionExpired: true,
					}
					return
				}
				httpResult <- insightHTTPResultMsg{requestID: requestID, err: err.Error()}
				return
			}
			httpResult <- insightHTTPResultMsg{requestID: requestID, response: &response}
		}()

		// Open the SSE subscription with a small retry loop. This must race
		// with the HTTP call so the request_id is already registered.
		result := startInsightWithRetry(ctx, m.client, chatID, requestID)
		started := insightStartedMsg{
			chatID:    result.chatID,
			requestID: result.requestID,
			stream:    result.stream,
		}
		if result.err != nil {
			if expired, ok := sessionExpiredFromError(result.err); ok {
				cancel()
				return expired
			}
			started.err = result.err.Error()
		}

		// Return a composite message carrying the started event, the HTTP
		// result channel, and the cancel func so Update can stash it.
		return insightStartPair{
			started:    started,
			httpResult: httpResult,
			cancel:     cancel,
		}
	}
}

// insightStartPair is an internal message used to hand off the SSE startup
// outcome, the channel that will deliver the HTTP response, and the cancel
// func that tears both down. The Update handler splits this into the two
// real messages and stashes the cancel func on the model.
type insightStartPair struct {
	started    insightStartedMsg
	httpResult <-chan insightHTTPResultMsg
	cancel     context.CancelFunc
}

func awaitInsightHTTPResult(ch <-chan insightHTTPResultMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// startInsightWithRetry opens the SSE subscription, retrying briefly while
// the backend registers the request_id. Backoff is exponential with a small
// jitter so a flock of clients does not synchronize. The retry loop respects
// ctx cancellation so quits and workspace switches do not block on a
// retry sleep.
func startInsightWithRetry(ctx context.Context, client *api.Client, chatID string, requestID string) startInsightResult {
	const (
		initialBackoff = 150 * time.Millisecond
		maxBackoff     = 1200 * time.Millisecond
		maxAttempts    = 6
	)

	if !sleepCtx(ctx, initialBackoff) {
		return startInsightResult{chatID: chatID, requestID: requestID, err: ctx.Err()}
	}

	var lastErr error
	backoff := initialBackoff
	for attempt := 0; attempt < maxAttempts; attempt++ {
		stream, err := client.OpenInsightStream(ctx, api.InsightStreamRequest{RequestID: requestID})
		if err != nil {
			lastErr = err
			if !sleepCtx(ctx, jittered(backoff)) {
				return startInsightResult{chatID: chatID, requestID: requestID, err: ctx.Err()}
			}
			backoff = nextBackoff(backoff, maxBackoff)
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
			if !sleepCtx(ctx, jittered(backoff)) {
				return startInsightResult{chatID: chatID, requestID: requestID, err: ctx.Err()}
			}
			backoff = nextBackoff(backoff, maxBackoff)
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

// sleepCtx blocks for d or until ctx is done. Returns false when ctx ended
// first so callers can short-circuit their retry loop.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// jittered returns d perturbed by up to ±25% so concurrent retriers do not
// synchronize their backoff intervals.
func jittered(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	// Use crypto-grade randomness via time.Now nanos as a cheap jitter
	// source; this does not need to be unpredictable, just decorrelated.
	nanos := time.Now().UnixNano()
	delta := (nanos % int64(d/2)) - int64(d/4)
	return d + time.Duration(delta)
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
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

		// Translate progress events into a status line + optional trace
		// dispatch. The status line tracks the agent's high-level phase
		// (Analyzing, Running...) while the trace dispatch records each
		// thought delta or tool lifecycle step.
		status := ""
		var trace *insightTraceDispatch
		switch event.Type {
		case "confirmation_required":
			// Surface as a dedicated message so the modal can take
			// focus. The SSE pump pauses here naturally: the agent
			// loop is blocked server-side until the client POSTs
			// to /requests/confirm, so no further events arrive
			// until the user decides.
			return confirmationRequiredMsg{
				requestID:         event.RequestID,
				confirmationID:    event.ConfirmationID,
				toolDisplayName:   firstNonEmpty(event.ToolDisplayName, event.ToolName),
				message:           event.Message,
				actionDescription: event.ActionDescription,
				timeoutSeconds:    event.TimeoutSeconds,
			}
		case "phase_update":
			status = strings.TrimSpace(event.Message)
			if status == "" {
				status = humanizePhase(event.Phase)
			}
		case "agent_thought", "reasoning":
			trace = &insightTraceDispatch{
				kind:            "thought",
				iteration:       event.Iteration,
				thoughtDelta:    event.Content,
				thoughtComplete: event.IsComplete,
			}
		case "tool_call_announced":
			trace = &insightTraceDispatch{
				kind: "step",
				step: chat.TraceStep{
					Kind:       chat.TraceToolCall,
					ToolCallID: event.ToolCallID,
					Title:      firstNonEmpty(event.ToolDisplayName, event.ToolName, "Tool call"),
					Iteration:  event.Iteration,
				},
			}
		case "tool_output":
			if event.PrivateOutput {
				break
			}
			trace = &insightTraceDispatch{
				kind:        "output",
				toolCallID:  event.ToolCallID,
				toolContent: event.Content,
			}
		case "tool_call_complete":
			trace = &insightTraceDispatch{
				kind:        "complete",
				toolCallID:  event.ToolCallID,
				toolContent: event.ResultPreview,
				toolStatus:  event.Status,
				toolMS:      event.DurationMS,
			}
		}

		return insightChunkMsg{
			chatID:    event.ChatID,
			requestID: event.RequestID,
			status:    status,
			trace:     trace,
			done:      event.Done,
		}
	}
}

// applyTraceDispatch routes a parsed SSE trace event to the chat model.
// Splitting this out keeps the Update switch readable and makes it easy
// to extend with new trace kinds.
func (m *Model) applyTraceDispatch(d *insightTraceDispatch) {
	if d == nil {
		return
	}
	switch d.kind {
	case "thought":
		m.chat.MergeStreamThoughtDelta(d.iteration, d.thoughtDelta, d.thoughtComplete)
	case "step":
		m.chat.AppendStreamTrace(d.step)
	case "output":
		m.chat.AppendStreamToolOutputContent(d.toolCallID, d.toolContent)
	case "complete":
		m.chat.PairToolOutput(d.toolCallID, d.toolContent, d.toolStatus, d.toolMS)
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

// hydrateModels seeds availableModels, defaultModelSlug, and selectedModel
// from the startup fetch. Resolution order for selectedModel:
//   - persisted config.SelectedModel if still present in availableModels
//   - backend recommended default (defModel)
//   - first available slug
//   - api.FallbackDefaultModel (with a synthetic single-entry list so the
//     UI still renders when the options fetch failed entirely)
func (m *Model) hydrateModels(models []api.ModelChoice, defModel string) {
	m.availableModels = models
	m.defaultModelSlug = strings.TrimSpace(defModel)

	persisted := strings.TrimSpace(m.config.SelectedModel)

	hasSlug := func(slug string) bool {
		for _, c := range m.availableModels {
			if c.Slug == slug {
				return true
			}
		}
		return false
	}

	switch {
	case persisted != "" && hasSlug(persisted):
		m.selectedModel = persisted
	case m.defaultModelSlug != "" && hasSlug(m.defaultModelSlug):
		m.selectedModel = m.defaultModelSlug
	case len(m.availableModels) > 0:
		m.selectedModel = m.availableModels[0].Slug
	default:
		// Options fetch failed and we have no list. Synthesize one so
		// the picker / badge are not empty.
		fallback := persisted
		if fallback == "" {
			fallback = api.FallbackDefaultModel
		}
		m.availableModels = []api.ModelChoice{{
			Provider: "openai",
			Slug:     fallback,
			Display:  fallback,
		}}
		m.selectedModel = fallback
		if m.defaultModelSlug == "" {
			m.defaultModelSlug = fallback
		}
	}

	m.chat.SetModel(m.modelDisplayName(m.selectedModel))
}

// modelDisplayName returns the user-facing label for slug, falling back
// to the slug itself when no matching ModelChoice is found.
func (m Model) modelDisplayName(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	for _, c := range m.availableModels {
		if c.Slug == slug {
			if d := strings.TrimSpace(c.Display); d != "" {
				return d
			}
			return c.Slug
		}
	}
	return slug
}

// persistSelectedModel writes the current selectedModel slug to
// ~/.papermap/config.yaml. Errors are intentionally swallowed: a failed
// write should not block the user from continuing the session.
func (m *Model) persistSelectedModel() {
	cfg := m.config
	cfg.SelectedModel = m.selectedModel
	if err := config.Save(cfg); err == nil {
		m.config = cfg
	}
}

// clearPersistedModel wipes the SelectedModel field from disk. Called on
// session expiry / logout so the next user does not inherit the prior
// account's model preference (which may not be in their tier).
func (m *Model) clearPersistedModel() {
	cfg := m.config
	cfg.SelectedModel = ""
	if err := config.Save(cfg); err == nil {
		m.config = cfg
	}
	m.selectedModel = ""
	m.availableModels = nil
	m.defaultModelSlug = ""
	m.chat.SetModel("")
}

// closeStream closes the SSE subscription without cancelling the parent
// ctx. Use this when SSE has reached its terminal state (complete / error)
// but the HTTP POST may still be in flight delivering the final answer.
func (m *Model) closeStream() {
	if m.stream == nil {
		return
	}
	_ = m.stream.Close()
	m.stream = nil
}

// cancelInsight tears down both the SSE subscription and the in-flight
// HTTP POST. Use this on user-initiated paths (Clear, switch workspace,
// session expiry, quit) where any pending answer should be discarded.
func (m *Model) cancelInsight() {
	if m.insightCancel != nil {
		m.insightCancel()
		m.insightCancel = nil
	}
	m.closeStream()
}

// resetInsightState clears per-request buffers and coordination flags. Call
// on any path that tears down an in-flight insight (error, clear, logout,
// session expiry, completion).
func (m *Model) resetInsightState() {
	m.pendingResponse = nil
	m.pendingRequestID = ""
	m.httpReceived = false
	m.sseComplete = false
	// Drop the cancel reference; the request has terminated (success or
	// failure) and any later teardown should be a no-op.
	m.insightCancel = nil
	// Drop any pending approval modal: the request that produced it
	// is gone, so allowing or denying it can no longer affect the
	// agent.
	m.closeDialog()
}

// tryFinalizeInsight renders the final assistant message once BOTH the HTTP
// body (the only source of the final answer) and the SSE `complete` sentinel
// have arrived. If either is still pending, it returns without rendering.
func (m Model) tryFinalizeInsight() (tea.Model, tea.Cmd) {
	if !m.httpReceived || !m.sseComplete {
		return m, nil
	}

	response := m.pendingResponse
	m.resetInsightState()
	m.chat.SetStreamStatus("")

	if response != nil {
		message := buildAssistantMessage(response)
		hasContent := strings.TrimSpace(message.Content) != "" ||
			message.Table != nil || message.Tile != nil || message.Chart != nil ||
			message.EmptyData || message.ChartType != ""
		if hasContent {
			m.chat.ReplaceLastAssistant([]chat.Message{message})
		} else {
			m.chat.CompleteStream()
		}
	} else {
		m.chat.CompleteStream()
	}

	return m, nil
}

// responseTable returns the table parsed from an HTTP InsightResponse's raw
// payload, or nil when no table is present.
func responseTable(response *api.InsightResponse) *api.InsightTable {
	if response == nil || response.Raw == nil {
		return nil
	}
	return api.ExtractTable(response.Raw)
}

// buildAssistantMessage converts an InsightResponse into a chat.Message,
// dispatching on chart_type. Tables and tiles parse directly from the
// preserved raw `data` JSON bytes so column/key order stays faithful to
// the backend response. Unsupported chart types fall through with the
// markdown narrative and a chart-type badge added by the renderer.
func buildAssistantMessage(response *api.InsightResponse) chat.Message {
	content := strings.TrimSpace(response.TextResponse)
	if content == "" {
		content = strings.TrimSpace(response.Thoughts)
	}

	chartType := strings.ToLower(strings.TrimSpace(response.ChartType))
	message := chat.Message{
		Role:      "alan",
		Content:   content,
		ChartType: chartType,
	}

	switch chartType {
	case "table":
		table := api.BuildDataRowsTable(response.RawDataJSON)
		if table != nil {
			message.Table = toChatTable(table)
		} else {
			// Fall back to legacy {columns, rows} extractor.
			if legacy := responseTable(response); legacy != nil {
				message.Table = toChatTable(legacy)
			} else {
				message.EmptyData = true
			}
		}

	case "tile":
		tile := api.BuildTile(response.RawDataJSON)
		if tile != nil {
			message.Tile = &chat.Tile{
				Label:        tile.Label,
				Value:        tile.Value,
				FormatConfig: response.VisualizationConfig,
			}
		} else {
			message.EmptyData = true
		}

	default:
		// Bar, line, pie, scatter, area, and radar render via the
		// charts package. Unknown types fall through to the chart-type
		// badge added by the chat renderer. The legacy extractor stays
		// in play as a final fallback so ad-hoc table-shaped payloads
		// remain visible.
		if charts.IsSupported(chartType) {
			table := api.BuildDataRowsTable(response.RawDataJSON)
			if table == nil {
				table = responseTable(response)
			}
			if table != nil {
				message.Chart = &chat.Chart{
					Type:   chartType,
					Table:  table,
					Config: response.Chart(),
				}
			} else {
				message.EmptyData = true
			}
		} else if legacy := responseTable(response); legacy != nil {
			message.Table = toChatTable(legacy)
		}
	}

	return message
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
	return nil, false
}
