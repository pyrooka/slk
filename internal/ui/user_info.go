package ui

import (
	"strconv"
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
	teamID, userID, channelID, kind, name, presence string
	requestID                                       uint64
	status                                          peerstatus.Status
	profile                                         core.UserProfile
	info                                            core.ConversationInfo
	loading                                         bool
	err                                             error
	offset                                          int
}

func (m *userInfoModal) Open(teamID, userID, name, presence string, status peerstatus.Status) {
	m.requestID++
	id := m.requestID
	*m = userInfoModal{teamID: teamID, userID: userID, kind: "user", name: name, presence: presence, status: status, loading: true, requestID: id}
}

func (m *userInfoModal) OpenConversation(teamID, channelID, name, kind string) {
	m.requestID++
	id := m.requestID
	*m = userInfoModal{teamID: teamID, channelID: channelID, name: name, kind: kind, loading: true, requestID: id}
}

func (m *userInfoModal) Close() {
	id := m.requestID
	*m = userInfoModal{requestID: id}
}

func (m *userInfoModal) IsVisible() bool { return m.userID != "" || m.channelID != "" }

func (m *userInfoModal) Scroll(delta, termWidth int) {
	innerWidth := max(1, min(80, termWidth-2)-4)
	m.offset = max(0, min(m.offset+delta, len(m.rows(innerWidth))-1))
}

func (m *userInfoModal) rows(innerWidth int) []string {
	if m.loading || m.err != nil {
		rows := wrappedProfileLines(m.name, innerWidth)
		if m.loading {
			label := "Loading profile…"
			if m.kind != "user" {
				label = "Loading conversation info…"
			}
			return append(rows, label)
		}
		if m.kind == "user" {
			return append(rows, "Profile unavailable")
		}
		return append(rows, wrappedProfileLines("Conversation info unavailable: "+m.err.Error(), innerWidth)...)
	}
	if m.kind != "user" {
		rows := wrappedProfileLines(m.name, innerWidth)
		add := func(label, value string) { rows = append(rows, userInfoFieldRows(label, value, innerWidth)...) }
		if m.kind == "group_dm" {
			names := make([]string, 0, len(m.info.Members))
			for _, member := range m.info.Members {
				name := member.Name
				if name == "" {
					name = member.ID
				}
				names = append(names, name)
			}
			count := "Unavailable"
			if m.info.HasMemberCount {
				count = strconv.Itoa(m.info.MemberCount)
			}
			if members := strings.Join(names, ", "); members != "" {
				if count == "Unavailable" {
					count = members
				} else {
					count += " · " + members
				}
			}
			add("Members", count)
		} else {
			add("Topic", m.info.Topic)
			add("Description", m.info.Description)
			count := "Unavailable"
			if m.info.HasMemberCount {
				count = strconv.Itoa(m.info.MemberCount)
			}
			add("Members", count)
			add("Creator", m.info.Creator)
		}
		return rows
	}
	name := m.profile.DisplayName
	if name == "" {
		name = m.name
	}
	rows := wrappedProfileLines(name, innerWidth)
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
		rows = append(rows, wrappedProfileLines(line, innerWidth)...)
	}
	add := func(label, value string) {
		rows = append(rows, userInfoFieldRows(label, value, innerWidth)...)
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
	return rows
}

// cleanProfileText keeps Slack-provided values from injecting terminal controls.
func cleanProfileText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func wrappedProfileLines(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	return strings.Split(messages.WordWrap(cleanProfileText(s), width), "\n")
}

// userInfoFieldRows wraps a labeled value and aligns continuation lines under
// the value column, keeping both soft wraps and embedded newlines readable.
func userInfoFieldRows(label, value string, width int) []string {
	if value == "" || width <= 0 {
		return nil
	}
	labelWidth := min(11, max(1, width-2))
	label = profileLine(strings.ReplaceAll(cleanProfileText(label), "\n", " "), labelWidth)
	prefix := label + strings.Repeat(" ", max(0, labelWidth-lipgloss.Width(label))) + " "
	prefixWidth := lipgloss.Width(prefix)
	wrapped := wrappedProfileLines(value, max(1, width-prefixWidth))
	if len(wrapped) == 0 {
		return nil
	}
	rows := []string{prefix + wrapped[0]}
	continuation := strings.Repeat(" ", prefixWidth)
	for _, line := range wrapped[1:] {
		rows = append(rows, continuation+line)
	}
	return rows
}

// profileLine removes terminal controls and truncates UI-owned single-line text.
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
	boxWidth := min(80, termWidth-2)
	innerWidth := boxWidth - 4 // border and one column of padding on either side
	rows := m.rows(innerWidth)
	visible := min(len(rows), termHeight-5) // title, spacer, footer, and borders
	start := min(m.offset, max(0, len(rows)-visible))
	pad := func(line string) string {
		return line + strings.Repeat(" ", max(0, innerWidth-lipgloss.Width(line)))
	}
	bg := styles.Background
	lines := make([]string, 0, visible+2)
	title := "User info"
	if m.kind == "group_dm" {
		title = "Group info"
	} else if m.kind != "user" {
		title = "Channel info"
	}
	lines = append(lines, lipgloss.NewStyle().Background(bg).Foreground(styles.Primary).Bold(true).Render(pad(profileLine(title, innerWidth))))
	bodyStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.TextPrimary)
	lines = append(lines, bodyStyle.Render(pad("")))
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
