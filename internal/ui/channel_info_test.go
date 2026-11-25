package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/sidebar"
)

func sidebarInfoApp(t *testing.T, selected string, err error) *App {
	t.Helper()
	a := newTestApp(t, withSize(160, 40), withActiveTeam("T1"), withChannels(
		sidebar.ChannelItem{ID: "C1", Name: "general", Type: "channel"},
		sidebar.ChannelItem{ID: "G1", Name: "priya, sam", Type: "group_dm"},
		sidebar.ChannelItem{ID: "D1", Name: "priya", Type: "dm", DMUserID: "U7"},
	))
	setChannelFuncsForTest(a, core.ChannelServiceFuncs{
		Info: func(_ context.Context, team ids.TeamID, ch ids.ChannelID) (core.ChannelInfo, error) {
			if team != "T1" || string(ch) != selected {
				t.Errorf("Info(%q, %q), want (T1, %s)", team, ch, selected)
			}
			return core.ChannelInfo{Topic: "ship &amp; tell", Purpose: "all hands", Members: 12,
				Created: time.Date(2024, 1, 2, 12, 0, 0, 0, time.Local)}, err
		},
	})
	a.focusedPanel = PanelSidebar
	a.sidebar.SelectByID(selected)
	if got := a.sidebar.SelectedID(); got != selected {
		t.Fatalf("precondition: selected %q, want %q", got, selected)
	}
	return a
}

func promptText(a *App) string { return ansi.Strip(a.confirmPrompt.View()) }

func TestSidebarK_DMOpensProfile(t *testing.T) {
	a := sidebarInfoApp(t, "D1", nil)
	_, cmd := a.Update(keyPress('K'))
	team, user := a.userProfile.Target()
	if a.mode != ModeUserProfile || team != "T1" || user != "U7" || cmd == nil {
		t.Fatalf("mode=%v target=(%q,%q) cmd nil=%v", a.mode, team, user, cmd == nil)
	}
}

func TestSidebarK_ChannelShowsInfo(t *testing.T) {
	for id, kind := range map[string]string{"C1": "Channel", "G1": "Group DM"} {
		a := sidebarInfoApp(t, id, nil)
		_, cmd := a.Update(keyPress('K'))
		if a.mode != ModeConfirm || !strings.Contains(promptText(a), "Loading") {
			t.Fatalf("%s: mode=%v, prompt:\n%s", id, a.mode, promptText(a))
		}
		a.Update(deliveredMsgs[ChannelInfoLoadedMsg](t, cmd)[0])
		for _, want := range []string{kind, "12 members · created Jan 2, 2024", "Topic: ship & tell", "Purpose: all hands"} {
			if !strings.Contains(promptText(a), want) {
				t.Errorf("%s: prompt lacks %q:\n%s", id, want, promptText(a))
			}
		}
		a.Update(keyPress('K'))
		if a.mode != ModeNormal || a.focusedPanel != PanelSidebar {
			t.Errorf("%s: after close mode=%v panel=%v", id, a.mode, a.focusedPanel)
		}
	}
}

func TestSidebarK_ChannelInfoError(t *testing.T) {
	a := sidebarInfoApp(t, "C1", errors.New("boom"))
	_, cmd := a.Update(keyPress('K'))
	a.Update(deliveredMsgs[ChannelInfoLoadedMsg](t, cmd)[0])
	if !strings.Contains(promptText(a), "boom") {
		t.Errorf("prompt:\n%s", promptText(a))
	}
}

// A result landing after the prompt closed, or under another prompt, is dropped.
func TestSidebarK_LateInfoDropped(t *testing.T) {
	a := sidebarInfoApp(t, "C1", nil)
	_, cmd := a.Update(keyPress('K'))
	msg := deliveredMsgs[ChannelInfoLoadedMsg](t, cmd)[0]
	a.Update(keyPress('q'))
	a.Update(msg)
	if a.confirmPrompt.IsVisible() {
		t.Fatal("late result reopened the prompt")
	}
	a.openConfirmPrompt("Quit?", "", nil)
	a.Update(msg)
	if !strings.Contains(promptText(a), "Quit?") {
		t.Errorf("late result replaced another prompt:\n%s", promptText(a))
	}
}

func TestSidebarK_NonConversationRowToasts(t *testing.T) {
	a := sidebarInfoApp(t, "C1", nil)
	a.sidebar.SelectThreadsRow()
	a.Update(keyPress('K'))
	if a.mode != ModeNormal || !strings.Contains(statusbarText(a), noSidebarInfoToast) {
		t.Errorf("mode=%v statusbar=%q", a.mode, statusbarText(a))
	}
}
