package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/messages"
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

func (m *userInfoModal) Scroll(delta, termWidth int) {
	innerWidth := max(1, min(64, termWidth-2)-4)
	m.offset = max(0, min(m.offset+delta, len(m.rows(innerWidth))-1))
}

func (m *userInfoModal) rows(innerWidth int) []string {
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
	if status := cleanProfileText(m.status.Summary(time.Now(), "15:04")); status != "" {
		rows = append(rows, strings.Split(messages.WordWrap(fmt.Sprintf("%-10s %s", "Status", status), innerWidth), "\n")...)
	}
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

// cleanProfileText keeps Slack-provided values from injecting terminal controls.
func cleanProfileText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// profileLine keeps other profile fields on one line; the status uses WordWrap.
func profileLine(s string, width int) string {
	s = cleanProfileText(s)
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
	rows := m.rows(innerWidth)
	visible := min(len(rows), termHeight-4) // title + footer + borders
	start := min(m.offset, max(0, len(rows)-visible))
	pad := func(line string) string {
		return line + strings.Repeat(" ", max(0, innerWidth-lipgloss.Width(line)))
	}
	bg := styles.Background
	lines := make([]string, 0, visible+2)
	lines = append(lines, lipgloss.NewStyle().Background(bg).Foreground(styles.Primary).Bold(true).Render(pad(profileLine("User info", innerWidth))))
	bodyStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.TextPrimary)
	for _, row := range rows[start : start+visible] {
		lines = append(lines, bodyStyle.Render(pad(profileLine(row, innerWidth))))
	}
	lines = append(lines, lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted).Render(pad(profileLine("j/k scroll · Esc close", innerWidth))))
	content := messages.ReapplyBgAfterResets(strings.Join(lines, "\n"), messages.BgANSI()+messages.FgANSI())
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(styles.Primary).BorderBackground(bg).
		Background(bg).Padding(0, 1).
		Render(content)
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
