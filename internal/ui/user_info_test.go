package ui

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

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
		sidebar.ChannelItem{ID: "G2", Type: "group_dm"},
		sidebar.ChannelItem{ID: "C1", Name: "general", Type: "channel"},
		sidebar.ChannelItem{ID: "P1", Name: "private-room", Type: "private"},
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
	loadingRows := strings.Join(a.userInfo.rows(60), "\n")
	if strings.Contains(loadingRows, "User ID") || !strings.Contains(loadingRows, "Loading profile") {
		t.Fatalf("partial profile shown while loading: %q", loadingRows)
	}
	result, ok := cmd().(UserProfileLoadedMsg)
	if !ok || team != "T1" || user != "U1" || result.UserID != "U1" {
		t.Fatalf("fetch result = %+v, requested %s/%s", result, team, user)
	}
	a.Update(result)
	if a.userInfo.loading || a.userInfo.profile.Email != "avery@example.com" {
		t.Fatalf("loaded popup = %+v", a.userInfo)
	}
	if !strings.Contains(strings.Join(a.userInfo.rows(60), "\n"), userInfoFieldRows("Name", "Avery Chen", 60)[0]) {
		t.Fatal("profile name does not use attribute formatting")
	}
	rows := a.userInfo.rows(60)
	if rows[len(rows)-1] != userInfoFieldRows("Handle", "avery", 60)[0] {
		t.Fatalf("handle is not the last profile attribute: %q", rows)
	}
	if !strings.Contains(strings.Join(rows, "\n"), userInfoFieldRows("Presence", "Active", 60)[0]) {
		t.Fatal("presence does not use attribute formatting")
	}
	plain := ansi.Strip(a.userInfo.ViewOverlay(a.width, a.height, strings.Repeat(" ", a.width)))
	for _, part := range []string{"Avery Chen", "Presence", "Active", "Handle", "avery", "Engineer", "avery@example.com"} {
		if !strings.Contains(plain, part) {
			t.Errorf("popup missing %q: %q", part, plain)
		}
	}
	a.Update(keyCode(tea.KeyEscape))
	if a.mode != ModeNormal || a.userInfo.IsVisible() || a.focusedPanel != PanelSidebar {
		t.Fatalf("closed: mode=%v visible=%v focus=%v", a.mode, a.userInfo.IsVisible(), a.focusedPanel)
	}
}

func TestUserInfoModalHasSpacerAfterTitle(t *testing.T) {
	a := userInfoTestApp(t)
	a.userInfo.Open("T1", "U1", "Avery", "", peerstatus.Status{})
	a.userInfo.loading = false
	a.userInfo.profile.DisplayName = "Avery Chen"
	lines := strings.Split(ansi.Strip(a.userInfo.renderBox(80, 20)), "\n")
	for i, line := range lines {
		if strings.Contains(line, "User info") {
			if i+2 >= len(lines) || strings.Trim(lines[i+1], " │") != "" || !strings.Contains(lines[i+2], "Avery Chen") {
				t.Fatalf("expected one blank row after title: %q", lines)
			}
			return
		}
	}
	t.Fatalf("title missing: %q", lines)
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
		{"group", "G1", PanelSidebar, false, true},
		{"channel", "C1", PanelSidebar, false, true},
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
			a.SetConversationInfoFetcher(func(_, _, _ string) (core.ConversationInfo, error) { return core.ConversationInfo{}, nil })
			cmd := dispatchModeKey(a, keyPress('I'))
			if got := a.mode == ModeUserInfo && cmd != nil; got != tc.want {
				t.Fatalf("opened=%v, want %v", got, tc.want)
			}
			if tc.want && a.userInfo.IsVisible() == false {
				t.Fatal("popup not visible")
			}
		})
	}
	a := userInfoTestApp(t)
	_ = dispatchModeKey(a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Fatalf("i mode = %v", a.mode)
	}
}

func TestConversationInfoOpenAndRender(t *testing.T) {
	for _, tc := range []struct {
		name  string
		id    string
		kind  string
		info  core.ConversationInfo
		want  []string
		title string
	}{
		{"group members", "G1", "group_dm", core.ConversationInfo{Members: []core.ConversationMember{{ID: "U1", Name: "Alice"}, {ID: "U2", Name: "Bob"}}, MemberCount: 2, HasMemberCount: true}, []string{"Group", "Members", "Alice", "Bob"}, "Group info"},
		{"unnamed group members", "G2", "group_dm", core.ConversationInfo{Members: []core.ConversationMember{{ID: "U4"}}, MemberCount: 1, HasMemberCount: true}, []string{"Members", "U4", "1"}, "Group info"},
		{"channel info", "C1", "channel", core.ConversationInfo{Topic: "Topic text", Description: "Purpose text", Creator: "Alice", MemberCount: 14, HasMemberCount: true}, []string{"general", "Topic text", "Purpose text", "Alice", "14"}, "Channel info"},
		{"missing channel count", "P1", "private", core.ConversationInfo{Creator: "U9"}, []string{"Unavailable", "U9"}, "Channel info"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := userInfoTestApp(t)
			a.sidebar.SelectByID(tc.id)
			var team, channel, kind string
			a.SetConversationInfoFetcher(func(tid, cid, typ string) (core.ConversationInfo, error) {
				team, channel, kind = tid, cid, typ
				return tc.info, nil
			})
			cmd := dispatchModeKey(a, keyPress('I'))
			if cmd == nil || a.mode != ModeUserInfo || a.userInfo.channelID != tc.id || a.userInfo.kind != tc.kind {
				t.Fatalf("open: cmd=%v mode=%v popup=%+v", cmd != nil, a.mode, a.userInfo)
			}
			loadingRows := strings.Join(a.userInfo.rows(80), "\n")
			if strings.Contains(loadingRows, "Members") || !strings.Contains(loadingRows, "Loading conversation info") {
				t.Fatalf("partial content shown while loading: %q", loadingRows)
			}
			msg, ok := cmd().(ConversationInfoLoadedMsg)
			if !ok || team != "T1" || channel != tc.id || kind != tc.kind {
				t.Fatalf("fetch msg=%+v requested %s/%s/%s", msg, team, channel, kind)
			}
			a.Update(msg)
			if a.userInfo.name != "" && !strings.Contains(strings.Join(a.userInfo.rows(76), "\n"), userInfoFieldRows("Name", a.userInfo.name, 76)[0]) {
				t.Errorf("conversation name does not use attribute formatting")
			}
			plain := ansi.Strip(a.userInfo.renderBox(100, 40))
			if !strings.Contains(plain, tc.title) {
				t.Errorf("title missing %q: %s", tc.title, plain)
			}
			for _, text := range tc.want {
				if !strings.Contains(plain, text) {
					t.Errorf("popup missing %q: %s", text, plain)
				}
			}
			if tc.info.Description != "" && (!strings.Contains(plain, "Description") || strings.Contains(plain, "Descripti…")) {
				t.Errorf("description label was truncated: %s", plain)
			}
			if tc.kind == "group_dm" {
				members := make([]string, 0, len(tc.info.Members))
				for _, member := range tc.info.Members {
					if member.Name != "" {
						members = append(members, member.Name)
					} else {
						members = append(members, member.ID)
					}
				}
				value := strconv.Itoa(tc.info.MemberCount) + " · " + strings.Join(members, ", ")
				wantRow := userInfoFieldRows("Members", value, 76)[0]
				rows := strings.Join(a.userInfo.rows(76), "\n")
				if !strings.Contains(rows, wantRow) {
					t.Errorf("members are not a single comma-separated list: %q", rows)
				}
				if len(members) > 0 && strings.Contains(rows, userInfoFieldRows("Member", members[0], 76)[0]) {
					t.Errorf("member is still listed separately: %q", rows)
				}
			}
		})
	}
}

func TestConversationDescriptionRemainsAvailableWhenScrolled(t *testing.T) {
	a := userInfoTestApp(t)
	a.sidebar.SelectByID("C1")
	a.SetConversationInfoFetcher(func(_, _, _ string) (core.ConversationInfo, error) {
		return core.ConversationInfo{Description: strings.Repeat("Detailed channel purpose text. ", 40) + "description end marker"}, nil
	})
	msg := dispatchModeKey(a, keyPress('I'))().(ConversationInfoLoadedMsg)
	a.Update(msg)
	rows := a.userInfo.rows(76)
	for i, row := range rows {
		if lipgloss.Width(row) > 76 {
			t.Errorf("row %d width %d exceeds 76", i, lipgloss.Width(row))
		}
	}
	if !strings.Contains(strings.Join(rows, "\n"), "description end marker") {
		t.Fatal("description was truncated from popup rows")
	}
	a.userInfo.Scroll(1000, 100)
	if !strings.Contains(ansi.Strip(a.userInfo.renderBox(100, 12)), "description end marker") {
		t.Fatal("scrolling could not reveal the full description")
	}
}

func TestConversationInfoShowsFetchErrors(t *testing.T) {
	a := userInfoTestApp(t)
	a.sidebar.SelectByID("C1")
	a.SetConversationInfoFetcher(func(_, _, _ string) (core.ConversationInfo, error) {
		return core.ConversationInfo{}, errors.New("denied")
	})
	msg := dispatchModeKey(a, keyPress('I'))().(ConversationInfoLoadedMsg)
	a.Update(msg)
	if !strings.Contains(strings.Join(a.userInfo.rows(80), "\n"), "denied") {
		t.Fatal("fetch error was hidden")
	}
}

func TestConversationInfoResultDroppedAfterNewSelection(t *testing.T) {
	a := userInfoTestApp(t)
	a.sidebar.SelectByID("G1")
	a.SetConversationInfoFetcher(func(_, _, _ string) (core.ConversationInfo, error) { return core.ConversationInfo{Topic: "stale"}, nil })
	first := dispatchModeKey(a, keyPress('I'))().(ConversationInfoLoadedMsg)
	a.Update(keyCode(tea.KeyEscape))
	a.sidebar.SelectByID("C1")
	second := dispatchModeKey(a, keyPress('I'))().(ConversationInfoLoadedMsg)
	if first.RequestID == second.RequestID {
		t.Fatal("request IDs reused")
	}
	a.Update(first)
	if a.userInfo.channelID != "C1" || a.userInfo.info.Topic != "" {
		t.Fatalf("stale result applied: %+v", a.userInfo)
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

func TestUserInfoFieldRowsWrapWithAlignedContinuations(t *testing.T) {
	a := userInfoTestApp(t)
	_ = dispatchModeKey(a, keyPress('I'))
	a.userInfo.loading = false
	a.userInfo.profile = core.UserProfile{
		DisplayName: "Avery Chen",
		Title:       "Engineering lead coordinating a very large team across several different offices",
		Email:       "first line\nsecond line continues with another very long part that must wrap",
	}
	a.userInfo.status = peerstatus.Status{Text: "Scheduling another week with the team and the whole organization"}
	const width = 30
	rows := a.userInfo.rows(width)
	for i, row := range rows {
		if lipgloss.Width(row) > width {
			t.Errorf("row %d width %d exceeds %d: %q", i, lipgloss.Width(row), width, row)
		}
	}
	assertFieldRows := func(label, value string) {
		t.Helper()
		want := userInfoFieldRows(label, value, width)
		start := -1
		for i, row := range rows {
			if len(want) > 0 && row == want[0] {
				start = i
				break
			}
		}
		if start < 0 || start+len(want) > len(rows) {
			t.Fatalf("%s field rows missing from:\n%s", label, strings.Join(rows, "\n"))
		}
		for i, line := range want {
			if rows[start+i] != line {
				t.Errorf("%s line %d = %q, want aligned %q", label, i, rows[start+i], line)
			}
		}
	}
	assertFieldRows("Title", a.userInfo.profile.Title)
	assertFieldRows("Email", a.userInfo.profile.Email)
	assertFieldRows("Status", a.userInfo.status.Summary(time.Now(), "15:04"))
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, "second line") || !strings.Contains(joined, "organization") {
		t.Fatalf("multiline values were lost:\n%s", joined)
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
