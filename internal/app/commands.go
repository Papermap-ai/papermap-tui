// Command catalog for the chat command palette. The palette is the single
// surface that exposes screen-level actions; this file keeps the catalog
// (id, title, hint, shortcut) close to the dispatch logic so adding a new
// command is one place to edit.
package app

import "github.com/papermap/papermap-tui/internal/ui/components/palette"

const (
	commandConversations   = "conversations"
	commandSwitchWorkspace = "switch-workspace"
	commandToggleThinking  = "toggle-thinking"
	commandClearSession    = "clear-session"
	commandQuit            = "quit"
)

// chatPaletteCommands returns the static command list for the chat
// palette. Order matters: it is the order shown to the user.
func chatPaletteCommands() []palette.Command {
	return []palette.Command{
		{
			ID:       commandConversations,
			Title:    "Conversations",
			Hint:     "Browse and load prior chats for this dashboard",
			Shortcut: "Ctrl+P",
		},
		{
			ID:       commandSwitchWorkspace,
			Title:    "Switch workspace",
			Hint:     "Change the active workspace",
			Shortcut: "Ctrl+W",
		},
		{
			ID:       commandToggleThinking,
			Title:    "Toggle thinking",
			Hint:     "Show or hide the assistant's reasoning trace",
			Shortcut: "Ctrl+T",
		},
		{
			ID:       commandClearSession,
			Title:    "Clear / new session",
			Hint:     "Drop the current chat and start fresh",
			Shortcut: "Ctrl+L",
		},
		{
			ID:       commandQuit,
			Title:    "Quit",
			Hint:     "Exit Papermap",
			Shortcut: "Ctrl+C",
		},
	}
}
