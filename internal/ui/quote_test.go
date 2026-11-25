package ui

import "testing"

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
