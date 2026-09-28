# Sidebar conversation info popup

**Date:** 2026-09-28
**Status:** DM, group and channel info implemented; selectable rows and copy planned

## Current DM behavior

In normal mode, focus the sidebar, select a one-to-one DM (including an app DM), and press `I` (Shift+i). A centered popup opens immediately with the selected person's name and a loading indicator. The selected row supplies `ChannelItem.DMUserID`; the active conversation and sidebar cursor do not change. `i` still enters insert mode.

When Slack replies, show the fields it returned: display name, real name, handle, pronouns, title, custom status, presence, time zone, email, phone, Slack user ID, and any labeled custom fields present in the response. Omit empty fields rather than implying Slack supplied them. Use the selected DM's live presence/DND state and the existing `peerstatus.Status` expiry/emoji rules for status; the fetched profile supplies the other details. On failure, retain the sidebar name and show `Profile unavailable` inside the popup. Do not save contact fields in SQLite.

`Esc` or `q` closes the popup and restores normal mode and sidebar focus. `j`/`k`, arrows, and the mouse wheel scroll long profiles within a height-capped box. Clicking outside dismisses it; clicking inside does nothing. Other keys cannot reach the sidebar while the popup is open. `I` does nothing when focus is elsewhere, the sidebar is hidden, or the cursor is on a section header, Threads, or Activity. Group DMs and channels use the same popup below.

Current layout (fields depend on what Slack returns):

```text
User info

Name         Avery Chen
Presence     Active
Title        Engineering lead
Pronouns     they/them
Status       🌴 Away until Monday
Email        avery@example.com

User ID      U123
Handle       avery
j/k scroll · Esc close
```

## Current DM data flow

- Add `UserInfo` to `KeyMap` (`I`, help text `sidebar user info`) and handle it in `handleNormalMode`, after the existing chord/reaction-navigation intercepts. Require `PanelSidebar`, a visible sidebar, and `SelectedItem()` with `Type == "dm" || Type == "app"` and a nonempty `DMUserID`. Do not mistake `SelectedID()` (a conversation ID) for a user ID.
- Add an exclusive `ModeUserInfo` to `Mode`, `modeHandlers`, and `IsModalOverlay`. Hold the popup's selected team/user IDs, loading/error state, profile and monotonically increasing request number in App-owned UI state. The new mode handler handles close and scrolling. Add its view to `applyOverlays`/`overlayActive`; use `overlay.DimmedOverlay` and existing styles. Register its box in `activeModalClickTarget` as a non-list modal. Cap both box width and height to the terminal; sanitize control characters and wrap long values with continuation lines aligned under the value column.
- Introduce a one-function port in `internal/core`, `UserProfileFetchFunc(teamID, userID string) (UserProfile, error)`, with a small `core.UserProfile` value (and labeled custom fields) rather than importing Slack into `internal/ui`. Expose an App setter. `cmd/slk` wires it to the existing `WorkspaceContext.Client.GetUserProfile(userID)` (`users.info`), resolving the **requested** team through `workspaceRouter.ByID(teamID)`, not whichever workspace is active when the request finishes. Map only display fields; do not introduce another network method or a persistent profile cache.
- Opening returns a Bubble Tea command that invokes the fetcher off the Update loop and sends `UserProfileLoadedMsg{TeamID, UserID, RequestID, Profile, Err}`. A new user-info reducer accepts it only while that popup is open and all three identifiers match. Closing, switching workspaces, or opening a newer request invalidates the old response. Leaving `ModeUserInfo` closes the popup, including when a global quit confirmation replaces it. A workspace switch already calls `SetMode(ModeNormal)`.

The existing `cache.User` has display name, presence and status but not real name, title, phone, or email. A full popup therefore needs the on-demand request. Slack can omit individual fields. In particular, Slack documents that `users.info` returns email only with `users:read.email` as well as `users:read`; an absent email is not an error. Custom fields are best-effort from `users.info`: Slack documents `users.profile.get` for complete custom fields, which would require another endpoint/scope and is outside this version. [Slack `users.info`](https://docs.slack.dev/reference/methods/users.info/), [Slack `users.profile.get`](https://docs.slack.dev/reference/methods/users.profile.get/).

## Existing DM checks

- `I` on a focused human DM and app DM opens the popup for `DMUserID`; `i` still enters insert mode. Headers and unfocused sidebar do nothing.
- An asynchronous result fills the correct popup; a result after Esc, a second open, or a workspace switch never displays another user's details. A failed request shows an error without losing the sidebar name.
- Narrow/short terminal, long Unicode/control-character fields, many returned fields, arrow/wheel scrolling, outside/inside clicks and quit confirmation keep the frame bounded and input routed to the popup.
- Verify the `cmd/slk` adapter maps optional fields and handles unavailable email/custom fields without extra requests. Run targeted `go test ./internal/ui ./cmd/slk`, then the normal build/vet/race/format checks before a PR.

## Group and channel extension (implemented)

### Behavior

With sidebar focus, `I` opens the **same centered, scrollable popup** for three row types:

| Selected row | Popup contents |
| --- | --- |
| Human or app DM | Existing user profile, unchanged. |
| Group DM (`group_dm`) | Group name and the returned members, including the signed-in user when present. List display names in a stable alphabetical order; show an ID when no name is known. If Slack returns only a count, show it without inventing a member list. |
| Public or private channel (`channel`, `private`) | Channel name, topic, description (Slack `purpose.value`), number of members, and creator (the channel's `creator` user, not the person who last edited the topic). Resolve the creator's display name if known; otherwise show the user ID. |

Empty topic or description fields are omitted. A missing or unreported member count is **Unavailable**, not `0`. The popup opens with the sidebar name and a loading row, then fills in details without selecting/opening the conversation or changing read state. Preserve `Esc`/`q`, wheel and key scrolling, click-outside dismissal, theme colors, control-character sanitization, and multiline value alignment. Unavailable Slack metadata stays distinguishable from an empty field; on fetch failure show an error rather than inventing values.

**Managed by:** Omitted for now at the user's request. The existing `conversations.info` response has a creator but no channel-manager list. Slack's [admin role-assignment API](https://docs.slack.dev/reference/methods/admin.roles.listAssignments/) needs an Enterprise org installation and admin authorization. Do not label the creator as a manager or silently infer managers. Add this row only if a reliable authorized source becomes available.

### Data flow

- `handleNormalMode` routes `I` by `SelectedItem().Type`: `DMUserID` for `dm`/`app`, **conversation ID** (`item.ID`) for `group_dm`, `channel` and `private`. Synthetic rows and other focused panels do nothing. The same modal/mode holds either target kind; `IsVisible` recognizes a user or a conversation.
- The on-demand `internal/core` conversation-info port is wired through `workspaceRouter.ByID(teamID)` in `cmd/slk`. `Client.GetConversationInfo(ctx, channelID)` requests `IncludeNumMembers: true` for channel topic, purpose, creator and count. A missing count displays as `Unavailable`, never `0`. The channel creator is not the topic/purpose editor.
- Use the existing paginated `Client.GetUsersInConversation(ctx, channelID)` for a group DM's complete member IDs. Resolve names from workspace-scoped cached users; fall back to IDs without making one profile request per member. If the members response is empty or fails, try `conversations.info` for its embedded members or count; do not treat join deltas in SQLite as a full list. Do not call `membership.Manager.EnsureFresh` on the Bubble Tea Update goroutine: its callback can synchronously call `Program.Send` and deadlock.
- Fetch in a Bubble Tea command. Deliver a typed result with `TeamID`, `ChannelID`, row kind and the popup's monotonic `RequestID`; the reducer ignores responses after close, a newer open, or a workspace switch. A channel ID must never be passed to the user-profile fetcher. Reuse the existing popup's bounded, aligned multiline rows to render channel text and member names.

### Checks before shipping

- Focused group DM shows all returned members (including self) by name, with ID fallback; incomplete join deltas are never presented as a full list. A channel and a private channel show distinct topic, description, member count and creator; missing `num_members` is not shown as zero.
- `I` still opens the correct DM profile; switching from group to channel (or to another workspace) while a request runs cannot display the first selection's data. Group/channel fetch errors remain visible inside the popup. Headers, synthetic rows, hidden sidebar and other focused panels still do nothing.
- Long topic, description and member names wrap within the modal; continuations align with their value column. Narrow screens remain bounded and scroll to the end. Test `cmd/slk` metadata mapping and the `include_num_members` request with an HTTP fixture, then run build, vet, race tests, lint and formatting checks.

## Planned selectable rows and copy

Keep the one popup/mode. Follow the keybinding view's selected-row treatment: a visible indicator (`▌`) and accent styling on the selected row. Every displayed **field** row is selectable, including name, presence, custom fields, the group member list, IDs and `Unavailable` counts. Wrapped value lines are selectable too, but each maps to its parent field; title, blank separators, footer and loading/error notices are not fields. During loading or after an error, the sidebar-provided Name is still selectable. Reset the selection to the first field on open; clamp it when an async result changes the rows.

`j`/`k`, Up/Down and the mouse wheel move the selected row, keep it visible and skip blank separators. Long descriptions remain readable by stepping through their continuation lines; keep the existing height/width bounds, sanitization and aligned wrapping, with space for the indicator. The overflow footer shows the selected row's position and total, plus `y copy` and the existing close hint. Clicking a field selects it without copying; clicking title, footer or padding does nothing; clicking outside still closes. Reuse `activeModalClickTarget` with a selectable popup and no click activation, as Help does; do not add search, tabs or another modal.

`y` copies only the selected field's **whole value**, never its label, highlight glyph, soft-wrap newlines or ellipsis. Preserve actual newlines in a multiline value, and strip terminal controls using the popup's existing sanitization. Copy exactly the displayed field value otherwise: for example, `Handle` copies `avery`, `Members` copies `2 · Alice, Bob`, and an unavailable count copies `Unavailable`. Use `App.clipboardWrite` and the existing copied toast; leave the popup and selection in place. Do nothing when no field is selectable, and do not send the value through a new I/O path in `internal/ui`.

### Checks before shipping

- Keyboard and wheel navigation highlight every field (including a wrapped continuation), skip spacers/notices, keep the selection visible in a short terminal and show the correct footer position. Clicking a field only selects it; clicking outside dismisses it without changing the underlying sidebar selection.
- With an injected clipboard writer, `y` copies the complete sanitized value from Name, a wrapped/multiline Description, the group Members list, a custom field and Handle, without the label or visual wrapping. Check the copied toast; no data row produces no copy command. `Esc`/`q` and the normal `i` binding keep their behavior.
- A late result after close, reopen or workspace switch cannot change selection or make another conversation's value copyable. Run targeted UI tests and the usual build, vet, race, lint and format checks.

## Scope

No new OAuth scopes, separate manager API, avatar downloads, profile edits, or persisted contact details. SQLite is a cache, never the authority over a successful Slack response.
