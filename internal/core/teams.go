// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"sort"
	"strings"
)

type teamRecord struct {
	ID         string            `json:"id"`
	Target     string            `json:"target"`
	Title      string            `json:"title"`
	Brief      string            `json:"brief"`
	Membership map[string]string `json:"membership"`
}

type TeamMember struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type TeamView struct {
	ID              string       `json:"id"`
	Target          string       `json:"target"`
	Title           string       `json:"title"`
	Brief           string       `json:"brief"`
	Status          string       `json:"status"`
	Members         []TeamMember `json:"members"`
	GrantsAuthority bool         `json:"grantsAuthority"`
}

func (s *Service) teams(actor, method string, p params) (any, error) {
	target, err := s.target(actor, p.Target)
	if err != nil {
		return nil, err
	}
	if method == "teams.list" {
		return s.listTeams(actor, target, p)
	}
	if !checkID(p.ID) {
		return nil, errors.New("team ID required")
	}
	key := target + "\x00" + p.ID
	if method == "teams.create" {
		return s.createTeam(actor, key, target, p)
	}
	team, ok := s.data.Teams[key]
	if !ok {
		return nil, errors.New("team unavailable")
	}
	if method == "teams.invite" || method == "teams.revoke" {
		return s.changeTeamInvitation(actor, method, key, team, p.To)
	}
	status := team.Membership[actor]
	if actor != "operator" && (status == "" || status == "revoked") {
		return nil, errors.New("team unavailable")
	}
	switch method {
	case "teams.get":
	case "teams.join", "teams.leave":
		if actor == "operator" {
			return nil, errors.New("enrolled team participant required")
		}
		status = "joined"
		if method == "teams.leave" {
			status = "left"
		}
		team.Membership[actor] = status
		s.data.Teams[key] = team
	default:
		return nil, errors.New("unknown team method")
	}
	return s.teamView(team, status), nil
}

func (s *Service) createTeam(actor, key, target string, p params) (any, error) {
	if actor != "operator" {
		return nil, errors.New("operator required")
	}
	if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Text) == "" || len(p.Title) > 512 || len(p.Text) > 8192 {
		return nil, errors.New("team title and brief required; max 512B title and 8KiB brief")
	}
	if existing, ok := s.data.Teams[key]; ok {
		if existing.Title != p.Title || existing.Brief != p.Text {
			return nil, errors.New("team ID already has a different brief")
		}
		return s.teamView(existing, ""), nil
	}
	team := teamRecord{ID: p.ID, Target: target, Title: p.Title, Brief: p.Text, Membership: map[string]string{}}
	s.data.Teams[key] = team
	s.data.SchemaVersion = 2
	return s.teamView(team, ""), nil
}

func (s *Service) teamView(team teamRecord, status string) TeamView {
	view := TeamView{ID: team.ID, Target: team.Target, Title: team.Title, Brief: team.Brief, Status: status, Members: []TeamMember{}}
	for identity, membership := range team.Membership {
		if membership != "joined" {
			continue
		}
		session := s.data.Sessions[identity]
		view.Members = append(view.Members, TeamMember{Identity: identity, Name: session.Name, Role: session.Role})
	}
	sort.Slice(view.Members, func(i, j int) bool { return view.Members[i].Identity < view.Members[j].Identity })
	return view
}
