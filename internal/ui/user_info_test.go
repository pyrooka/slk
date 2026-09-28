package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/peerstatus"
	"github.com/gammons/slk/internal/ui/sidebar"
	"github.com/gammons/slk/internal/ui/styles"
)

func userInfoTestApp(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t, withActiveTeam("T1"), withChannels(
		sidebar.ChannelItem{ID: "D1", Name: "Avery", Type: "dm", DMUserID: "U1", Presence: "active"},
		sidebar.ChannelItem{ID: "D2", Name: "Build Bot", Type: "app", DMUserID: "U2"},
		sidebar.ChannelItem{ID: "G1", Name: "Group", Type: "group_dm"},
		sidebar.ChannelItem{ID: "C1", Name: "general", Type: "channel"},
	))
	a.sidebar.SelectByID("D1")
	if item, ok := a.sidebar.SelectedItem(); !ok || item.DMUserID != "U1" {
		t.Fatalf("selected = %+v, ok=%v", item, ok)
	}
	return a
}

func TestUserInfoOpenAndLoad(t *testing.T) {
	a := userInfoTestApp(t)
	var team, user string
	a.SetUserProfileFetcher(func(tid, uid string) (core.UserProfile, error) {
		team, user = tid, uid
		return core.UserProfile{DisplayName: "Avery Chen", RealName: "Avery C.", Handle: "avery", Title: "Engineer", Email: "avery@example.com"}, nil
	})
	_, cmd := a.Update(keyPress('I'))
	if a.mode != ModeUserInfo || cmd == nil || !a.overlayActive() {
		t.Fatalf("open: mode=%v cmd=%v overlay=%v", a.mode, cmd != nil, a.overlayActive())
	}
	if a.userInfo.name != "Avery" || !a.userInfo.loading {
		t.Fatalf("initial popup = %+v", a.userInfo)
	}
	result, ok := cmd().(UserProfileLoadedMsg)
	if !ok || team != "T1" || user != "U1" || result.UserID != "U1" {
		t.Fatalf("fetch result = %+v, requested %s/%s", result, team, user)
	}
	a.Update(result)
	if a.userInfo.loading || a.userInfo.profile.Email != "avery@example.com" {
		t.Fatalf("loaded popup = %+v", a.userInfo)
	}
	plain := ansi.Strip(a.userInfo.ViewOverlay(a.width, a.height, strings.Repeat(" ", a.width)))
	for _, part := range []string{"Avery Chen", "@avery", "Engineer", "avery@example.com"} {
		if !strings.Contains(plain, part) {
			t.Errorf("popup missing %q: %q", part, plain)
		}
	}
	a.Update(keyCode(tea.KeyEscape))
	if a.mode != ModeNormal || a.userInfo.IsVisible() || a.focusedPanel != PanelSidebar {
		t.Fatalf("closed: mode=%v visible=%v focus=%v", a.mode, a.userInfo.IsVisible(), a.focusedPanel)
	}
}

func TestUserInfoOnlyOpensForSidebarDM(t *testing.T) {
	for _, tc := range []struct {
		name  string
		id    string
		focus Panel
		hide  bool
		want  bool
	}{
		{"human", "D1", PanelSidebar, false, true},
		{"app", "D2", PanelSidebar, false, true},
		{"group", "G1", PanelSidebar, false, false},
		{"channel", "C1", PanelSidebar, false, false},
		{"other focus", "D1", PanelMessages, false, false},
		{"hidden", "D1", PanelSidebar, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := userInfoTestApp(t)
			a.sidebar.SelectByID(tc.id)
			a.focusedPanel = tc.focus
			if tc.hide {
				a.sidebarVisible = false
			}
			a.SetUserProfileFetcher(func(_, _ string) (core.UserProfile, error) { return core.UserProfile{}, nil })
			cmd := dispatchModeKey(a, keyPress('I'))
			if got := a.mode == ModeUserInfo && cmd != nil; got != tc.want {
				t.Fatalf("opened=%v, want %v", got, tc.want)
			}
			if tc.want && a.userInfo.userID == "" {
				t.Fatal("missing selected user ID")
			}
		})
	}
	a := userInfoTestApp(t)
	_ = dispatchModeKey(a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Fatalf("i mode = %v", a.mode)
	}
}

func TestUserInfoDropsOldAndFailedResults(t *testing.T) {
	a := userInfoTestApp(t)
	a.SetUserProfileFetcher(func(_, _ string) (core.UserProfile, error) { return core.UserProfile{}, errors.New("offline") })
	first := dispatchModeKey(a, keyPress('I'))().(UserProfileLoadedMsg)
	a.Update(keyCode(tea.KeyEscape))
	a.Update(first)
	if a.userInfo.IsVisible() {
		t.Fatal("closed popup reopened by result")
	}
	second := dispatchModeKey(a, keyPress('I'))().(UserProfileLoadedMsg)
	if first.RequestID == second.RequestID {
		t.Fatal("requests reused generation")
	}
	a.Update(first)
	if !a.userInfo.loading || a.userInfo.userID != "U1" {
		t.Fatalf("stale result changed popup: %+v", a.userInfo)
	}
	a.Update(second)
	if a.userInfo.loading || a.userInfo.err == nil || a.userInfo.name != "Avery" {
		t.Fatalf("failed popup = %+v", a.userInfo)
	}
	a.SetMode(ModeNormal) // workspace switch / global interrupt
	if a.userInfo.IsVisible() {
		t.Fatal("SetMode left popup visible")
	}
}

func TestUserInfoModalCapturesClicksAndKeys(t *testing.T) {
	a := userInfoTestApp(t)
	_ = dispatchModeKey(a, keyPress('I'))
	selected := a.sidebar.SelectedID()
	_, _ = a.Update(keyPress('i'))
	if a.mode != ModeUserInfo || a.sidebar.SelectedID() != selected {
		t.Fatal("normal-mode key leaked through popup")
	}
	_, _ = a.Update(tea.MouseClickMsg{X: a.width / 2, Y: a.height / 2, Button: tea.MouseLeft})
	if a.mode != ModeUserInfo {
		t.Fatal("inside click closed popup")
	}
	_, _ = a.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if a.mode != ModeNormal || a.sidebar.SelectedID() != selected {
		t.Fatal("outside click did not close popup without changing sidebar selection")
	}
}

func TestUserInfoPopupBackgroundCoversBorderTextAndPadding(t *testing.T) {
	a := userInfoTestApp(t)
	_ = dispatchModeKey(a, keyPress('I'))
	box := a.userInfo.renderBox(80, 25)
	canvas := lipgloss.NewCanvas(lipgloss.Width(box), lipgloss.Height(box))
	canvas.Compose(lipgloss.NewLayer(box))
	for y := 0; y < lipgloss.Height(box); y++ {
		for x := 0; x < lipgloss.Width(box); x++ {
			cell := canvas.CellAt(x, y)
			if cell != nil && cell.Width > 0 && !colorEqual(cell.Style.Bg, styles.Background) {
				t.Errorf("cell (%d,%d) %q background = %v, want popup background", x, y, cell.Content, cell.Style.Bg)
			}
		}
	}
}

func TestUserInfoLongStatusWrapsAndScrollsToEnd(t *testing.T) {
	a := userInfoTestApp(t)
	a.width, a.height = 42, 8
	_ = dispatchModeKey(a, keyPress('I'))
	a.userInfo.loading = false
	a.userInfo.status = peerstatus.Status{Text: strings.Repeat("Scheduling another week with the team ", 4) + "FINAL-MARKER"}
	var frames []string
	for i := 0; i < 20; i++ {
		box := a.userInfo.renderBox(42, 8)
		if lipgloss.Width(box) > 42 || lipgloss.Height(box) > 8 {
			t.Fatalf("status box exceeds 42x8: %dx%d", lipgloss.Width(box), lipgloss.Height(box))
		}
		frames = append(frames, ansi.Strip(box))
		_ = dispatchModeKey(a, keyCode(tea.KeyDown))
	}
	got := strings.Join(frames, "\n")
	if !strings.Contains(got, "Status") || !strings.Contains(got, "FINAL-MARKER") || strings.Contains(got, "…") {
		t.Fatalf("status not fully visible by scrolling:\n%s", got)
	}
}

func TestUserInfoRenderBoundedAndScrollable(t *testing.T) {
	a := userInfoTestApp(t)
	_ = dispatchModeKey(a, keyPress('I'))
	a.userInfo.loading = false
	a.userInfo.profile = core.UserProfile{DisplayName: "Avery\x1b]2;hijack\a\nChen", Email: strings.Repeat("中", 80)}
	for i := 0; i < 25; i++ {
		a.userInfo.profile.Fields = append(a.userInfo.profile.Fields, core.ProfileField{Label: "Field", Value: "Value"})
	}
	w, h := 24, 12
	box := a.userInfo.renderBox(w, h)
	if lipgloss.Width(box) > w || len(strings.Split(box, "\n")) > h {
		t.Fatalf("box exceeds %dx%d: %dx%d\n%s", w, h, lipgloss.Width(box), len(strings.Split(box, "\n")), box)
	}
	if strings.Contains(box, "\x1b]2;") || strings.Contains(box, "\a") {
		t.Fatal("control characters leaked into box")
	}
	for i := 0; i < 20; i++ {
		_ = dispatchModeKey(a, keyCode(tea.KeyDown))
	}
	if a.userInfo.offset == 0 {
		t.Fatal("down did not scroll")
	}
	_ = dispatchModeKey(a, keyPress('q'))
	if a.mode != ModeNormal {
		t.Fatalf("q mode = %v", a.mode)
	}
}
