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
	offset, selected                                int
}

type userInfoLine struct {
	text       string
	value      string
	selectable bool
}

const userInfoListTopOffset = 3

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

func (m *userInfoModal) contentRows(width int) []userInfoLine {
	var rows []userInfoLine
	add := func(label, value string) {
		value = cleanProfileText(value)
		for _, text := range userInfoFieldRows(label, value, width) {
			rows = append(rows, userInfoLine{text: text, value: value, selectable: true})
		}
	}
	addText := func(text string) {
		for _, line := range wrappedProfileLines(text, width) {
			rows = append(rows, userInfoLine{text: line})
		}
	}

	if m.loading || m.err != nil {
		add("Name", m.name)
		if m.loading {
			label := "Loading profile…"
			if m.kind != "user" {
				label = "Loading conversation info…"
			}
			addText(label)
		} else if m.kind == "user" {
			addText("Profile unavailable")
		} else {
			addText("Conversation info unavailable: " + m.err.Error())
		}
		return rows
	}

	if m.kind != "user" {
		add("Name", m.name)
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
	add("Name", name)
	presence := ""
	if m.presence != "" {
		presence = strings.ToUpper(m.presence[:1]) + m.presence[1:]
	}
	add("Presence", presence)
	add("Real name", m.profile.RealName)
	add("Title", m.profile.Title)
	add("Pronouns", m.profile.Pronouns)
	add("Status", m.status.Summary(time.Now(), "15:04"))
	add("Time zone", m.profile.TimeZone)
	add("Email", m.profile.Email)
	add("Phone", m.profile.Phone)
	for _, f := range m.profile.Fields {
		add(f.Label, f.Value)
	}
	rows = append(rows, userInfoLine{})
	add("User ID", m.userID)
	add("Handle", m.profile.Handle)
	return rows
}

func (m *userInfoModal) rows(width int) []string {
	lines := m.contentRows(width)
	rows := make([]string, len(lines))
	for i, line := range lines {
		rows[i] = line.text
	}
	return rows
}

func (m *userInfoModal) rowWidth(termWidth int) int {
	return max(1, min(80, termWidth-2)-5)
}

func (m *userInfoModal) SelectedValue(termWidth int) (string, bool) {
	rows := m.contentRows(m.rowWidth(termWidth))
	if m.selected < 0 || m.selected >= len(rows) || !rows[m.selected].selectable {
		return "", false
	}
	return rows[m.selected].value, true
}

func (m *userInfoModal) MoveSelection(delta, termWidth, termHeight int) {
	if delta == 0 {
		return
	}
	rows := m.contentRows(m.rowWidth(termWidth))
	direction := 1
	steps := delta
	if delta < 0 {
		direction = -1
		steps = -delta
	}
	index := m.selected
	for step := 0; step < steps; step++ {
		for {
			index += direction
			if index < 0 || index >= len(rows) {
				return
			}
			if rows[index].selectable {
				m.selected = index
				m.keepSelectedVisible(rows, termHeight)
				break
			}
		}
	}
}

func (m *userInfoModal) ClickRow(termWidth, termHeight, localY int) bool {
	row := localY - userInfoListTopOffset
	rows := m.contentRows(m.rowWidth(termWidth))
	visible := min(len(rows), max(0, termHeight-5))
	start := min(m.offset, max(0, len(rows)-visible))
	index := start + row
	if row < 0 || row >= visible || index >= len(rows) || !rows[index].selectable {
		return false
	}
	m.selected = index
	m.keepSelectedVisible(rows, termHeight)
	return true
}

func (m *userInfoModal) ClampSelection(termWidth, termHeight int) {
	rows := m.contentRows(m.rowWidth(termWidth))
	if m.selected >= 0 && m.selected < len(rows) && rows[m.selected].selectable {
		m.keepSelectedVisible(rows, termHeight)
		return
	}
	for i := max(0, m.selected); i < len(rows); i++ {
		if rows[i].selectable {
			m.selected = i
			m.keepSelectedVisible(rows, termHeight)
			return
		}
	}
	for i := min(m.selected, len(rows)-1); i >= 0; i-- {
		if rows[i].selectable {
			m.selected = i
			m.keepSelectedVisible(rows, termHeight)
			return
		}
	}
	m.selected = -1
	m.offset = 0
}

func (m *userInfoModal) keepSelectedVisible(rows []userInfoLine, termHeight int) {
	visible := min(len(rows), max(0, termHeight-5))
	if visible == 0 {
		return
	}
	maxOffset := max(0, len(rows)-visible)
	if m.selected < m.offset {
		m.offset = m.selected
	} else if m.selected >= m.offset+visible {
		m.offset = m.selected - visible + 1
	}
	m.offset = max(0, min(m.offset, maxOffset))
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
	innerWidth := boxWidth - 4       // border and one column of padding on either side
	rowWidth := max(1, innerWidth-1) // reserve a column for the selection indicator
	rows := m.contentRows(rowWidth)
	visible := min(len(rows), max(0, termHeight-5)) // title, spacer, footer, and borders
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
	selectedStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.Primary).Bold(true)
	indicatorStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.Accent).Bold(true)
	lines = append(lines, bodyStyle.Render(pad("")))
	for i, row := range rows[start : start+visible] {
		indicator := bodyStyle.Render(" ")
		rowStyle := bodyStyle
		if row.selectable && start+i == m.selected {
			indicator = indicatorStyle.Render("▌")
			rowStyle = selectedStyle
		}
		text := profileLine(row.text, rowWidth)
		text += strings.Repeat(" ", max(0, rowWidth-lipgloss.Width(text)))
		lines = append(lines, indicator+rowStyle.Render(text))
	}
	footer := "j/k move · y copy · Esc close"
	position, totalSelectable := 0, 0
	for i, row := range rows {
		if row.selectable {
			totalSelectable++
			if i <= m.selected {
				position++
			}
		}
	}
	if visible < len(rows) {
		positionText := strconv.Itoa(position) + "/" + strconv.Itoa(totalSelectable)
		footer = "j/k move · y copy · " + positionText + " · Esc close"
		if lipgloss.Width(footer) > innerWidth {
			footer = "j/k · y copy · " + positionText + " · Esc"
		}
	}
	lines = append(lines, lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted).Render(pad(profileLine(footer, innerWidth))))
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
