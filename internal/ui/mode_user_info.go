package ui

import (
	tea "charm.land/bubbletea/v2"
)

func handleUserInfoMode(a *App, msg tea.KeyMsg) tea.Cmd {
	switch normalizeFinderKey(msg) {
	case "esc", "q":
		a.SetMode(ModeNormal)
	case "j", "down":
		a.userInfo.Scroll(1, a.width)
	case "k", "up":
		a.userInfo.Scroll(-1, a.width)
	}
	return nil
}
