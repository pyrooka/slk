package ui

import tea "charm.land/bubbletea/v2"

var reduceUserInfo reducerFunc = func(a *App, msg tea.Msg) (tea.Cmd, bool) {
	switch m := msg.(type) {
	case UserProfileLoadedMsg:
		if a.mode != ModeUserInfo || a.activeTeamID != m.TeamID || a.userInfo.teamID != m.TeamID ||
			a.userInfo.userID != m.UserID || a.userInfo.requestID != m.RequestID {
			return nil, true
		}
		a.userInfo.profile = m.Profile
		a.userInfo.err = m.Err
	case ConversationInfoLoadedMsg:
		if a.mode != ModeUserInfo || a.activeTeamID != m.TeamID || a.userInfo.teamID != m.TeamID ||
			a.userInfo.channelID != m.ChannelID || a.userInfo.kind != m.Kind || a.userInfo.requestID != m.RequestID {
			return nil, true
		}
		a.userInfo.info = m.Info
		a.userInfo.err = m.Err
	default:
		return nil, false
	}
	a.userInfo.loading = false
	a.userInfo.offset = 0
	return nil, true
}
