// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"errors"
	"strings"
)

func (s *Service) listBoard(target string, p params) (any, error) {
	if p.Topic != "" && !validBoardTopic(p.Topic) {
		return nil, errors.New("unsupported board topic filter")
	}
	if p.Order == "" {
		p.Order = "oldest"
	}
	if p.Order != "oldest" && p.Order != "newest" {
		return nil, errors.New("board order must be oldest or newest")
	}
	if p.Limit == 0 {
		p.Limit = 10
	}
	if p.Limit < 1 || p.Limit > 20 {
		return nil, errors.New("board limit must be 1..20")
	}
	posts := []BoardPost{}
	bytesUsed, next := 0, ""
	start := p.Cursor == ""
	for offset := range len(s.data.Board) {
		index := offset
		if p.Order == "newest" {
			index = len(s.data.Board) - 1 - offset
		}
		post := s.data.Board[index]
		if post.Target != target {
			continue
		}
		if !start {
			start = post.ID == p.Cursor
			continue
		}
		if !matchesBoardPost(post, p) {
			continue
		}
		encoded, _ := json.Marshal(post)
		if len(posts) > 0 && (len(posts) == p.Limit || bytesUsed+len(encoded) > 512<<10) {
			next = posts[len(posts)-1].ID
			break
		}
		posts = append(posts, post)
		bytesUsed += len(encoded)
	}
	if !start {
		return nil, errors.New("cursor unavailable")
	}
	return map[string]any{"posts": posts, "nextCursor": next, "order": p.Order}, nil
}

func matchesBoardPost(post BoardPost, p params) bool {
	return (p.Topic == "" || p.Topic == post.Topic) &&
		(p.Query == "" || strings.Contains(strings.ToLower(post.Title+"\n"+post.Text), strings.ToLower(p.Query)))
}
