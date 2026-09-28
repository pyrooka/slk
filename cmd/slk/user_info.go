package main

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/core"
)

// userProfileFetcher uses the requested workspace, not the workspace that
// happens to be active when the network command executes.
func userProfileFetcher(router *workspaceRouter) core.UserProfileFetchFunc {
	return func(teamID, userID string) (core.UserProfile, error) {
		wctx := router.ByID(teamID)
		if wctx == nil || wctx.Client == nil {
			return core.UserProfile{}, errors.New("workspace unavailable")
		}
		u, err := wctx.Client.GetUserProfile(userID)
		if err != nil {
			return core.UserProfile{}, err
		}
		if u == nil {
			return core.UserProfile{}, errors.New("user profile unavailable")
		}
		p := core.UserProfile{
			DisplayName: u.Profile.DisplayName,
			RealName:    u.Profile.RealName,
			Handle:      u.Name,
			Pronouns:    u.Profile.Pronouns,
			Title:       u.Profile.Title,
			TimeZone:    u.TZLabel,
			Email:       u.Profile.Email,
			Phone:       u.Profile.Phone,
		}
		if p.RealName == "" {
			p.RealName = u.RealName
		}
		if p.DisplayName == "" {
			p.DisplayName = p.RealName
		}
		if p.TimeZone == "" {
			p.TimeZone = u.TZ
		}
		fields := u.Profile.FieldsMap()
		keys := make([]string, 0, len(fields))
		for id := range fields {
			keys = append(keys, id)
		}
		slices.Sort(keys)
		for _, id := range keys {
			field := fields[id]
			if field.Label != "" && field.Value != "" {
				p.Fields = append(p.Fields, core.ProfileField{Label: field.Label, Value: field.Value})
			}
		}
		return p, nil
	}
}

func conversationInfoFetcher(router *workspaceRouter, db *cache.DB) core.ConversationInfoFetchFunc {
	return func(teamID, channelID, kind string) (core.ConversationInfo, error) {
		wctx := router.ByID(teamID)
		if wctx == nil || wctx.Client == nil {
			return core.ConversationInfo{}, errors.New("workspace unavailable")
		}
		nameByID := make(map[string]string)
		if db != nil {
			if users, err := db.ListUsers(teamID); err == nil {
				for _, user := range users {
					name := user.DisplayName
					if name == "" {
						name = user.Name
					}
					if name != "" {
						nameByID[user.ID] = name
					}
				}
			}
		}
		if kind == "group_dm" {
			ids, membersErr := wctx.Client.GetUsersInConversation(context.Background(), channelID)
			memberCount := len(ids)
			hasMemberCount := membersErr == nil
			if membersErr != nil || len(ids) == 0 {
				ch, err := wctx.Client.GetConversationInfo(context.Background(), channelID)
				if err != nil {
					if membersErr != nil {
						return core.ConversationInfo{}, membersErr
					}
					return core.ConversationInfo{}, err
				}
				if ch == nil {
					return core.ConversationInfo{}, errors.New("conversation info unavailable")
				}
				if len(ids) == 0 {
					ids = ch.Members
				}
				memberCount = len(ids)
				if memberCount == 0 {
					memberCount = ch.NumMembers
				}
				hasMemberCount = memberCount > 0 || membersErr == nil
				if !hasMemberCount {
					return core.ConversationInfo{}, membersErr
				}
			}
			info := core.ConversationInfo{MemberCount: memberCount, HasMemberCount: hasMemberCount, Members: make([]core.ConversationMember, 0, len(ids))}
			for _, id := range ids {
				info.Members = append(info.Members, core.ConversationMember{ID: id, Name: nameByID[id]})
			}
			slices.SortFunc(info.Members, func(a, b core.ConversationMember) int {
				if c := strings.Compare(strings.ToLower(memberDisplayName(a)), strings.ToLower(memberDisplayName(b))); c != 0 {
					return c
				}
				return strings.Compare(a.ID, b.ID)
			})
			return info, nil
		}
		ch, err := wctx.Client.GetConversationInfo(context.Background(), channelID)
		if err != nil {
			return core.ConversationInfo{}, err
		}
		if ch == nil {
			return core.ConversationInfo{}, errors.New("conversation info unavailable")
		}
		info := core.ConversationInfo{Topic: ch.Topic.Value, Description: ch.Purpose.Value}
		if ch.Creator != "" {
			info.Creator = nameByID[ch.Creator]
			if info.Creator == "" {
				info.Creator = ch.Creator
			}
		}
		if ch.NumMembers > 0 {
			info.MemberCount, info.HasMemberCount = ch.NumMembers, true
		}
		return info, nil
	}
}

func memberDisplayName(member core.ConversationMember) string {
	if member.Name != "" {
		return member.Name
	}
	return member.ID
}
