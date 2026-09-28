package main

import (
	"errors"
	"slices"

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
