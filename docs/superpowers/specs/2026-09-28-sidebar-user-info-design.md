# Sidebar user info popup

**Date:** 2026-09-28
**Status:** Proposed

## Behavior

In normal mode, focus the sidebar, select a one-to-one DM (including an app DM), and press `I` (Shift+i). A centered popup opens immediately with the selected person's name and a loading indicator. The selected row supplies `ChannelItem.DMUserID`; the active conversation and sidebar cursor do not change. `i` still enters insert mode.

When Slack replies, show the fields it returned: display name, real name, @handle, pronouns, title, custom status, presence, time zone, email, phone, Slack user ID, and any labeled custom fields present in the response. Omit empty fields rather than implying Slack supplied them. Use the selected DM's live presence/DND state and the existing `peerstatus.Status` expiry/emoji rules for status; the fetched profile supplies the other details. On failure, retain the sidebar name and show `Profile unavailable` inside the popup. Do not save contact fields in SQLite.

`Esc` or `q` closes the popup and restores normal mode and sidebar focus. `j`/`k`, arrows, and the mouse wheel scroll long profiles within a height-capped box. Clicking outside dismisses it; clicking inside does nothing. Other keys cannot reach the sidebar while the popup is open. `I` is a no-op when focus is elsewhere or the sidebar is hidden; on a non-DM row, section header, Threads or Activity row it is also a no-op. Group DMs have no single user to describe.

Example (fields depend on what Slack returns):

```text
╭─ User info ────────────────────────╮
│ Avery Chen                         │
│ @avery · Active                    │
│ Title       Engineering lead       │
│ Pronouns    they/them              │
│ Status      🌴 Away until Monday    │
│ Email       avery@example.com      │
│ Phone       +1 555 0100            │
│ Time zone   Pacific Time           │
│ User ID     U123                    │
│                                    │
│ j/k scroll · Esc close             │
╰────────────────────────────────────╯
```

## Data flow

- Add `UserInfo` to `KeyMap` (`I`, help text `sidebar user info`) and handle it in `handleNormalMode`, after the existing chord/reaction-navigation intercepts. Require `PanelSidebar`, a visible sidebar, and `SelectedItem()` with `Type == "dm" || Type == "app"` and a nonempty `DMUserID`. Do not mistake `SelectedID()` (a conversation ID) for a user ID.
- Add an exclusive `ModeUserInfo` to `Mode`, `modeHandlers`, and `IsModalOverlay`. Hold the popup's selected team/user IDs, loading/error state, profile and monotonically increasing request number in App-owned UI state. The new mode handler handles close and scrolling. Add its view to `applyOverlays`/`overlayActive`; use `overlay.DimmedOverlay` and existing styles. Register its box in `activeModalClickTarget` as a non-list modal. Cap both box width and height to the terminal; flatten control characters and truncate long values before drawing.
- Introduce a one-function port in `internal/core`, `UserProfileFetchFunc(teamID, userID string) (UserProfile, error)`, with a small `core.UserProfile` value (and labeled custom fields) rather than importing Slack into `internal/ui`. Expose an App setter. `cmd/slk` wires it to the existing `WorkspaceContext.Client.GetUserProfile(userID)` (`users.info`), resolving the **requested** team through `workspaceRouter.ByID(teamID)`, not whichever workspace is active when the request finishes. Map only display fields; do not introduce another network method or a persistent profile cache.
- Opening returns a Bubble Tea command that invokes the fetcher off the Update loop and sends `UserProfileLoadedMsg{TeamID, UserID, RequestID, Profile, Err}`. A new user-info reducer accepts it only while that popup is open and all three identifiers match. Closing, switching workspaces, or opening a newer request invalidates the old response. Leaving `ModeUserInfo` closes the popup, including when a global quit confirmation replaces it. A workspace switch already calls `SetMode(ModeNormal)`.

The existing `cache.User` has display name, presence and status but not real name, title, phone, or email. A full popup therefore needs the on-demand request. Slack can omit individual fields. In particular, Slack documents that `users.info` returns email only with `users:read.email` as well as `users:read`; an absent email is not an error. Custom fields are best-effort from `users.info`: Slack documents `users.profile.get` for complete custom fields, which would require another endpoint/scope and is outside this version. [Slack `users.info`](https://docs.slack.dev/reference/methods/users.info/), [Slack `users.profile.get`](https://docs.slack.dev/reference/methods/users.profile.get/).

## Checks before shipping

- `I` on a focused human DM and app DM opens the popup for `DMUserID`; `i` still enters insert mode. Channels, group DMs, headers and unfocused sidebar do nothing.
- An asynchronous result fills the correct popup; a result after Esc, a second open, or a workspace switch never displays another user's details. A failed request shows an error without losing the sidebar name.
- Narrow/short terminal, long Unicode/control-character fields, many returned fields, arrow/wheel scrolling, outside/inside clicks and quit confirmation keep the frame bounded and input routed to the popup.
- Verify the `cmd/slk` adapter maps optional fields and handles unavailable email/custom fields without extra requests. Run targeted `go test ./internal/ui ./cmd/slk`, then the normal build/vet/race/format checks before a PR.

## Scope

No sidebar changes, avatar download, profile editing, guaranteed custom-field retrieval, new OAuth scopes, or persistent storage of email/phone. Those can follow if users need them; the popup still displays every standard detail returned by the current `users.info` call.
