package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/overlay"
	"github.com/gammons/slk/internal/ui/peerstatus"
	"github.com/gammons/slk/internal/ui/styles"
)

// userInfoModal keeps a single on-demand profile; requestID survives Close so
// a late result from an earlier opening can never populate a later one.
type userInfoModal struct {
	teamID, userID, name, presence string
	requestID                      uint64
	status                         peerstatus.Status
	profile                        core.UserProfile
	loading                        bool
	err                            error
	offset                         int
}

func (m *userInfoModal) Open(teamID, userID, name, presence string, status peerstatus.Status) {
	m.requestID++
	id := m.requestID
	*m = userInfoModal{teamID: teamID, userID: userID, name: name, presence: presence, status: status, loading: true, requestID: id}
}

func (m *userInfoModal) Close() {
	id := m.requestID
	*m = userInfoModal{requestID: id}
}

func (m *userInfoModal) IsVisible() bool { return m.userID != "" }

func (m *userInfoModal) Scroll(delta int) {
	m.offset = max(0, min(m.offset+delta, len(m.rows())-1))
}

func (m *userInfoModal) rows() []string {
	name := m.profile.DisplayName
	if name == "" {
		name = m.name
	}
	rows := []string{name}
	line := ""
	if m.profile.Handle != "" {
		line = "@" + m.profile.Handle
	}
	if m.presence != "" {
		if line != "" {
			line += " · "
		}
		line += strings.ToUpper(m.presence[:1]) + m.presence[1:]
	}
	if line != "" {
		rows = append(rows, line)
	}
	add := func(label, value string) {
		if value != "" {
			rows = append(rows, fmt.Sprintf("%-10s %s", profileLine(label, 10), value))
		}
	}
	add("Real name", m.profile.RealName)
	add("Title", m.profile.Title)
	add("Pronouns", m.profile.Pronouns)
	add("Status", m.status.Summary(time.Now(), "15:04"))
	add("Time zone", m.profile.TimeZone)
	add("Email", m.profile.Email)
	add("Phone", m.profile.Phone)
	add("User ID", m.userID)
	for _, f := range m.profile.Fields {
		add(f.Label, f.Value)
	}
	if m.loading {
		rows = append(rows, "Loading profile…")
	}
	if m.err != nil {
		rows = append(rows, "Profile unavailable")
	}
	return rows
}

// profileLine removes terminal controls and forces Slack-provided values onto
// one line before ANSI-aware truncation; the UI never interprets profile text.
func profileLine(s string, width int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

func (m *userInfoModal) renderBox(termWidth, termHeight int) string {
	if !m.IsVisible() || termWidth < 8 || termHeight < 5 {
		return ""
	}
	boxWidth := min(64, termWidth-2)
	innerWidth := boxWidth - 4 // border and one column of padding on either side
	rows := m.rows()
	visible := min(len(rows), termHeight-4) // title + footer + borders
	start := min(m.offset, max(0, len(rows)-visible))
	lines := make([]string, 0, visible+2)
	lines = append(lines, lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render(profileLine("User info", innerWidth)))
	for _, row := range rows[start : start+visible] {
		lines = append(lines, profileLine(row, innerWidth))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(styles.TextMuted).Render(profileLine("j/k scroll · Esc close", innerWidth)))
	for i, line := range lines {
		lines[i] = line + strings.Repeat(" ", max(0, innerWidth-lipgloss.Width(line)))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(styles.Primary).
		Background(styles.Background).Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func (m *userInfoModal) BoxSize(termWidth, termHeight int) (int, int) {
	box := m.renderBox(termWidth, termHeight)
	if box == "" {
		return 0, 0
	}
	return lipgloss.Width(box), lipgloss.Height(box)
}

func (m *userInfoModal) ViewOverlay(termWidth, termHeight int, background string) string {
	box := m.renderBox(termWidth, termHeight)
	if box == "" {
		return background
	}
	return overlay.DimmedOverlay(termWidth, termHeight, background, box, 0.5)
}
