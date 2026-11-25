package ui

import (
	"testing"

	"github.com/gammons/slk/internal/ui/messages"
)

func TestQuoteSelected_PrependsAboveDraftAndEntersInsert(t *testing.T) {
	a := newTestApp(t, withMessages(testMessageItems(1)...))
	focusMessages(t, a)
	msg, _ := a.messagepane.SelectedMessage()
	a.compose.SetValue("draft")

	a.handleNormalMode(keyPress('>'))

	want := "> " + msg.Text + "\n\ndraft"
	if got := a.compose.Value(); got != want {
		t.Errorf("compose = %q, want %q", got, want)
	}
	if a.mode != ModeInsert {
		t.Errorf("mode = %v, want insert", a.mode)
	}
}

func TestQuoteSelected_EmptyComposerMultiline(t *testing.T) {
	a := newTestApp(t, withMessages(testMessageItems(1)...))
	focusMessages(t, a)
	a.messagepane.UpdateMessageInPlace(messages.MessageItem{TS: mustSelectedTS(a), Text: "a\nb"})

	a.handleNormalMode(keyPress('>'))

	if got, want := a.compose.Value(), "> a\n> b\n\n"; got != want {
		t.Errorf("compose = %q, want %q", got, want)
	}
}

func TestQuoteSelected_ComposeDisabledLeavesDraftAlone(t *testing.T) {
	a := newTestApp(t, withMessages(testMessageItems(1)...))
	focusMessages(t, a)
	a.composeDisabled = true
	a.compose.SetValue("draft")

	a.handleNormalMode(keyPress('>'))

	if got := a.compose.Value(); got != "draft" {
		t.Errorf("compose = %q, want draft untouched", got)
	}
	if a.mode == ModeInsert {
		t.Error("entered insert mode on a disabled composer")
	}
}

func mustSelectedTS(a *App) string {
	m, _ := a.messagepane.SelectedMessage()
	return m.TS
}

func TestQuoteSelected_ThreadReplyGoesToThreadComposer(t *testing.T) {
	a := newTestApp(t)
	focusThreadPanel(t, a)
	a.compose.SetValue("main")

	a.handleNormalMode(keyPress('>'))

	if got := a.threadCompose.Value(); got == "" || got[:2] != "> " {
		t.Errorf("thread composer = %q, want a quote", got)
	}
	if got := a.compose.Value(); got != "main" {
		t.Errorf("channel composer = %q, want untouched", got)
	}
}
