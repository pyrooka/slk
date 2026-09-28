package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/statusbar"
)

func handleUserInfoMode(a *App, msg tea.KeyMsg) tea.Cmd {
	switch normalizeFinderKey(msg) {
	case "esc", "q":
		a.SetMode(ModeNormal)
	case "j", "down":
		a.userInfo.MoveSelection(1, a.width, a.height)
	case "k", "up":
		a.userInfo.MoveSelection(-1, a.width, a.height)
	case "y":
		value, ok := a.userInfo.SelectedValue(a.width)
		if !ok {
			return nil
		}
		n := len([]rune(value))
		return tea.Batch(a.clipboardWrite(value), func() tea.Msg { return statusbar.CopiedMsg{N: n} })
	}
	return nil
}
