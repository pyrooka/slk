# Sidebar conversation info popup

**Date:** 2026-09-28
**Status:** DM profile implemented; group and channel info proposed

## Current DM behavior

In normal mode, focus the sidebar, select a one-to-one DM (including an app DM), and press `I` (Shift+i). A centered popup opens immediately with the selected person's name and a loading indicator. The selected row supplies `ChannelItem.DMUserID`; the active conversation and sidebar cursor do not change. `i` still enters insert mode.

When Slack replies, show the fields it returned: display name, real name, @handle, pronouns, title, custom status, presence, time zone, email, phone, Slack user ID, and any labeled custom fields present in the response. Omit empty fields rather than implying Slack supplied them. Use the selected DM's live presence/DND state and the existing `peerstatus.Status` expiry/emoji rules for status; the fetched profile supplies the other details. On failure, retain the sidebar name and show `Profile unavailable` inside the popup. Do not save contact fields in SQLite.

`Esc` or `q` closes the popup and restores normal mode and sidebar focus. `j`/`k`, arrows, and the mouse wheel scroll long profiles within a height-capped box. Clicking outside dismisses it; clicking inside does nothing. Other keys cannot reach the sidebar while the popup is open. `I` does nothing when focus is elsewhere, the sidebar is hidden, or the cursor is on a section header, Threads, or Activity. Group DMs and channels are the next step below.

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

## Planned group and channel extension

### Behavior

With sidebar focus, `I` opens the **same centered, scrollable popup** for three row types:

| Selected row | Popup contents |
| --- | --- |
| Human or app DM | Existing user profile, unchanged. |
| Group DM (`group_dm`) | Group name and every current member, including the signed-in user. List display names in a stable alphabetical order; show an ID when no name is known. Show a member count based on the complete returned list. |
| Public or private channel (`channel`, `private`) | Channel name, topic, description (Slack `purpose.value`), number of members, and creator (the channel's `creator` user, not the person who last edited the topic). Resolve the creator's display name if known; otherwise show the user ID. |

Empty topic or description fields are omitted. A missing or unreported member count is **Unavailable**, not `0`. The popup opens with the sidebar name and a loading row, then fills in details without selecting/opening the conversation or changing read state. Preserve `Esc`/`q`, wheel and key scrolling, click-outside dismissal, theme colors, control-character sanitization, and multiline value alignment. Unavailable Slack metadata stays distinguishable from an empty field; on fetch failure show an error rather than inventing values.

**Managed by:** Omitted for now at the user's request. The existing `conversations.info` response has a creator but no channel-manager list. Slack's [admin role-assignment API](https://docs.slack.dev/reference/methods/admin.roles.listAssignments/) needs an Enterprise org installation and admin authorization. Do not label the creator as a manager or silently infer managers. Add this row only if a reliable authorized source becomes available.

### Data flow

- Route `I` in `handleNormalMode` by `SelectedItem().Type`: keep the `DMUserID` path for `dm`/`app`; use the **conversation ID** (`item.ID`) for `group_dm`, `channel`, and `private`. No popup on synthetic rows or while another panel has focus. Keep one modal/mode and existing row renderer; add a conversation kind and selected channel ID rather than another modal package. Update its current `IsVisible` check (`userID != ""`) to recognize either target kind.
- Add an on-demand conversation-info port in `internal/core` and wire it from `cmd/slk` using `workspaceRouter.ByID(teamID)`. Return UI-neutral channel details or a group member list. Use `Client.GetConversationInfo(ctx, channelID)` for channel topic, purpose, creator and `num_members`. Its current wrapper does **not** request `include_num_members`; set `IncludeNumMembers: true` in that wrapper before interpreting the count. If Slack omits the count, show Unavailable. Do not use `Topic.Creator`/`Purpose.Creator` for the channel creator.
- Use the existing paginated `Client.GetUsersInConversation(ctx, channelID)` for a group DM's complete member IDs. Resolve names from workspace-scoped cached users; fall back to IDs without making one profile request per member. If offline, show `cache.ListChannelMembers` **only** when `GetChannelMembershipMeta` confirms a full snapshot, and label the list `Cached` because members may have changed since it was fetched. The cache can otherwise contain only join deltas. Do not call `membership.Manager.EnsureFresh` on the Bubble Tea Update goroutine: its callback can synchronously call `Program.Send` and deadlock.
- Fetch in a Bubble Tea command. Deliver a typed result with `TeamID`, `ChannelID`, row kind and the popup's monotonic `RequestID`; the reducer ignores responses after close, a newer open, or a workspace switch. A channel ID must never be passed to the user-profile fetcher. Reuse the existing popup's bounded, aligned multiline rows to render channel text and member names.

### Checks before shipping

- Focused group DM shows all returned members (including self) by name, with ID fallback; cached membership is labeled `Cached` and incomplete join deltas are never presented as a full list. A channel and a private channel show distinct topic, description, member count and creator; missing `num_members` is not shown as zero.
- `I` still opens the correct DM profile; switching from group to channel (or to another workspace) while a request runs cannot display the first selection's data. Group/channel fetch errors remain visible inside the popup. Headers, synthetic rows, hidden sidebar and other focused panels still do nothing.
- Long topic, description and member names wrap within the modal; continuations align with their value column. Narrow screens remain bounded and scroll to the end. Test `cmd/slk` metadata mapping and the `include_num_members` request with an HTTP fixture, then run build, vet, race tests, lint and formatting checks.

## Scope

No new OAuth scopes, separate manager API, avatar downloads, profile edits, or persisted contact details. SQLite is a fallback cache, never the authority over a successful Slack response.
