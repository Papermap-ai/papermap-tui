package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	screenLanding         screen = "landing"
	screenLogin           screen = "login"
	screenChat            screen = "chat"
	screenWorkspacePicker screen = "workspace_picker"
)

type startupMsg struct {
	config        config.Config
	authenticated bool
	err           error
}

type Model struct {
	width         int
	height        int
	screen        screen
	config        config.Config
	authenticated bool
	workspaceName string
	startupErr    error
	client        *api.Client
	theme         theme.Theme
	landing       landing.Model
	login         uitauth.Model
	chat          chat.Model
	workspace     workspace.Model
	store         *auth.TokenStore
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

	return Model{
		screen:        screenLanding,
		workspaceName: "Unified Workspace",
		theme:         theme.Default(),
		landing:       landing.NewModel(),
		login:         uitauth.NewModel(),
		chat:          chat.NewModel(),
		workspace:     workspace.NewModel(),
		store:         store,
	}, nil
}

func (m Model) Init() tea.Cmd {
	return m.loadStartup()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startupMsg:
		m.config = msg.config
		m.authenticated = msg.authenticated
		m.startupErr = msg.err
		if msg.err == nil {
			client, err := api.NewClient(msg.config.APIURL, nil, m.store)
			if err != nil {
				m.startupErr = err
			} else {
				m.client = client
			}
		}
		if m.authenticated {
			m.screen = screenChat
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
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

		authenticated := false
		cred, err := m.store.Load()
		switch {
		case err == nil:
			authenticated = cred.Valid()
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return startupMsg{config: cfg, err: err}
		}

		return startupMsg{config: cfg, authenticated: authenticated}
	}
}

func (m Model) handleEscape() Model {
	switch m.screen {
	case screenLogin, screenWorkspacePicker:
		m.screen = screenLanding
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
	case screenLogin:
		return m.login.View(m.theme, m.width)
	case screenChat:
		return m.chat.View(m.theme, m.workspaceName, m.width)
	case screenWorkspacePicker:
		return m.workspace.View(m.theme, m.width)
	default:
		return m.landing.View(m.theme, m.width, m.authenticated, m.workspaceName)
	}
}

func (m Model) frame(content string) string {
	styled := m.theme.App.Render(content)
	if m.width <= 0 || m.height <= 0 {
		return styled
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styled)
}
