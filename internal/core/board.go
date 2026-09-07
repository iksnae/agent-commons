// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *Service) board(actor, method string, p params) (any, error) {
	target, err := s.target(actor, p.Target)
	if err != nil {
		return nil, err
	}
	switch method {
	case "board.post":
		if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Text) == "" || p.IdempotencyKey == "" || len(p.Title) > 512 || len(p.Text) > 8192 || len(p.Evidence) > 8192 {
			return nil, errors.New("title, text and idempotencyKey required; title max 512B, text/evidence max 8KiB")
		}
		if !validBoardTopic(p.Topic) {
			return nil, errors.New("topic must be one of: " + strings.Join(BoardTopics(), ", "))
		}
		key := actor + "\x00board.post\x00" + p.IdempotencyKey
		if id, ok := s.data.Keys[key]; ok {
			for _, post := range s.data.Board {
				if post.ID == id {
					if post.Target != target || post.Title != p.Title || post.Text != p.Text || post.Topic != p.Topic || post.Evidence != p.Evidence || post.ReplyTo != p.ReplyTo {
						return nil, errors.New("idempotency key payload conflict")
					}
					return post, nil
				}
			}
		}
		if p.ReplyTo != "" {
			found := false
			for _, post := range s.data.Board {
				if post.ID == p.ReplyTo && post.Target == target {
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("reply parent unavailable")
			}
		}
		post := BoardPost{ID: randomID(), Target: target, Author: actor, Topic: p.Topic, Title: p.Title, Text: p.Text, Evidence: p.Evidence, ReplyTo: p.ReplyTo, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		s.data.Board = append(s.data.Board, post)
		s.data.Keys[key] = post.ID
		return post, nil
	case "board.get":
		for _, post := range s.data.Board {
			if post.ID == p.ID && post.Target == target {
				return post, nil
			}
		}
		return nil, errors.New("post unavailable")
	case "board.list":
		if p.Topic != "" && !validBoardTopic(p.Topic) {
			return nil, errors.New("unsupported board topic filter")
		}
		limit := p.Limit
		if limit == 0 {
			limit = 10
		}
		if limit < 1 || limit > 20 {
			return nil, errors.New("board limit must be 1..20")
		}
		posts := []BoardPost{}
		bytesUsed := 0
		start := p.Cursor == ""
		next := ""
		for _, post := range s.data.Board {
			if post.Target != target {
				continue
			}
			if !start {
				if post.ID == p.Cursor {
					start = true
				}
				continue
			}
			if p.Topic != "" && p.Topic != post.Topic || p.Query != "" && !strings.Contains(strings.ToLower(post.Title+"\n"+post.Text), strings.ToLower(p.Query)) {
				continue
			}
			encoded, _ := json.Marshal(post)
			if len(posts) > 0 && (len(posts) == limit || bytesUsed+len(encoded) > 512<<10) {
				next = posts[len(posts)-1].ID
				break
			}
			posts = append(posts, post)
			bytesUsed += len(encoded)
		}
		if !start {
			return nil, errors.New("cursor unavailable")
		}
		return map[string]any{"posts": posts, "nextCursor": next}, nil
	default:
		return nil, errors.New("unknown board method")
	}
}
