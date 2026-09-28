package ui

import tea "charm.land/bubbletea/v2"

var reduceUserInfo reducerFunc = func(a *App, msg tea.Msg) (tea.Cmd, bool) {
	m, ok := msg.(UserProfileLoadedMsg)
	if !ok {
		return nil, false
	}
	if a.mode != ModeUserInfo || a.activeTeamID != m.TeamID || a.userInfo.teamID != m.TeamID ||
		a.userInfo.userID != m.UserID || a.userInfo.requestID != m.RequestID {
		return nil, true
	}
	a.userInfo.profile = m.Profile
	a.userInfo.err = m.Err
	a.userInfo.loading = false
	a.userInfo.offset = 0
	return nil, true
}
