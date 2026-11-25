// internal/ui/channel_info.go
//
// K on the sidebar: the selected DM's profile, or a read-only prompt
// with the selected channel's info.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/bubbles/confirmprompt"
	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// noSidebarInfoToast answers K on a row that is not a conversation
// (Threads, Activity, a section header).
const noSidebarInfoToast = "No info for this row"

func (a *App) openSidebarInfo() tea.Cmd {
	item, ok := a.sidebar.SelectedItem()
	if !ok {
		return toastWithClear(a, noSidebarInfoToast, 2*time.Second)
	}
	if item.DMUserID != "" {
		return a.openProfileOf(item.DMUserID)
	}
	return a.openChannelInfo(item)
}

// openChannelInfo opens the info prompt for item and returns its fetch.
func (a *App) openChannelInfo(item sidebar.ChannelItem) tea.Cmd {
	a.openConfirmChoice(channelInfoChoice(item, "Loading…"))
	svc, seq, teamID := a.channels, a.confirmPromptSeq, a.activeTeamID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := svc.Info(ctx, ids.TeamID(teamID), ids.ChannelID(item.ID))
		return ChannelInfoLoadedMsg{Seq: seq, Item: item, Info: info, Err: err}
	}
}

// applyChannelInfo fills the prompt the fetch was made for, if it is
// still the one showing.
func (a *App) applyChannelInfo(m ChannelInfoLoadedMsg) {
	if m.Seq != a.confirmPromptSeq || !a.confirmPrompt.IsVisible() {
		return
	}
	if m.Err != nil {
		a.confirmPrompt.OpenChoice(channelInfoChoice(m.Item, "Couldn't load channel info: "+m.Err.Error()))
		return
	}
	facts := []string{}
	switch m.Info.Members {
	case 0:
	case 1:
		facts = append(facts, "1 member")
	default:
		facts = append(facts, fmt.Sprintf("%d members", m.Info.Members))
	}
	if !m.Info.Created.IsZero() {
		facts = append(facts, "created "+m.Info.Created.Format("Jan 2, 2006"))
	}
	lines := []string{}
	if len(facts) > 0 {
		lines = append(lines, strings.Join(facts, " · "))
	}
	user := func(id string) (string, bool) {
		n, ok := a.userNames[id]
		return n, ok && n != ""
	}
	channel := func(id string) (string, bool) {
		name, _, ok := a.channels.Lookup(ids.ChannelID(id))
		return name, ok
	}
	for _, f := range []struct{ label, text string }{{"Topic", m.Info.Topic}, {"Purpose", m.Info.Purpose}} {
		if f.text != "" {
			lines = append(lines, f.label+": "+messages.FlattenMrkdwn(f.text, user, channel))
		}
	}
	a.confirmPrompt.OpenChoice(channelInfoChoice(m.Item, lines...))
}

func channelInfoChoice(item sidebar.ChannelItem, lines ...string) confirmprompt.Choice {
	kind := "Channel"
	switch item.Type {
	case "private":
		kind = "Private channel"
	case "group_dm":
		kind = "Group DM"
	case "dm", "app":
		kind = "Direct message"
	}
	return confirmprompt.Choice{
		Title: messages.ChannelGlyph(item.Type) + " " + item.Name,
		Body:  strings.Join(append([]string{kind}, lines...), "\n"),
		Buttons: []confirmprompt.Button{{
			Label: "close",
			Hint:  "K / esc / q",
			Keys:  key.NewBinding(key.WithKeys("K", "esc", "q")),
		}},
	}
}
