// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"sort"
)

type TeamSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type TeamPage struct {
	Teams      []TeamSummary `json:"teams"`
	NextCursor string        `json:"nextCursor"`
}

func (s *Service) listTeams(actor, target string, p params) (TeamPage, error) {
	page := TeamPage{Teams: []TeamSummary{}}
	limit := p.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 20 {
		return page, errors.New("team limit must be 1..20")
	}
	visible := []TeamSummary{}
	for _, team := range s.data.Teams {
		if team.Target != target {
			continue
		}
		status := team.Membership[actor]
		if actor != "operator" && (status == "" || status == "revoked") {
			continue
		}
		visible = append(visible, TeamSummary{ID: team.ID, Title: team.Title, Status: status})
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	start := 0
	if p.Cursor != "" {
		index := sort.Search(len(visible), func(i int) bool { return visible[i].ID >= p.Cursor })
		if index == len(visible) || visible[index].ID != p.Cursor {
			return page, errors.New("team cursor unavailable; restart listing")
		}
		start = index + 1
	}
	end := min(start+limit, len(visible))
	page.Teams = append(page.Teams, visible[start:end]...)
	if end < len(visible) {
		page.NextCursor = visible[end-1].ID
	}
	return page, nil
}
