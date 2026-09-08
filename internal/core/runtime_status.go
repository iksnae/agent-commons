// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"sort"
	"time"
)

type RuntimeQueueStatus struct {
	Identity      string `json:"identity"`
	Runtime       string `json:"runtime"`
	Mode          string `json:"mode"`
	Busy          bool   `json:"busy"`
	NativeBound   bool   `json:"nativeBound"`
	Ready         int    `json:"ready"`
	WaitingTeam   int    `json:"waitingTeam"`
	BlockedPolicy int    `json:"blockedPolicy"`
	ManualPending int    `json:"manualPending"`
	Running       int    `json:"running"`
	Interrupted   int    `json:"interrupted"`
	Failed        int    `json:"failed"`
	Canceled      int    `json:"canceled"`
	Acknowledged  int    `json:"acknowledged"`
	Completed     int    `json:"completed"`
}

type RuntimeStatusPage struct {
	Supervisor SupervisorHealth     `json:"supervisor"`
	Build      BuildIdentity        `json:"build"`
	Sessions   []RuntimeQueueStatus `json:"sessions"`
	NextCursor string               `json:"nextCursor"`
	Notice     string               `json:"notice"`
}

func (s *Service) runtimeStatus(actor string, p params, now time.Time) (any, error) {
	if actor != "operator" && p.SessionID != "" && p.SessionID != actor {
		return nil, errors.New("runtime status limited to this identity")
	}
	if p.Target != "" && actor != "operator" && p.Target != s.data.Sessions[actor].Target {
		return nil, errors.New("target forbidden")
	}
	if p.Limit < 0 || p.Limit > 100 {
		return nil, errors.New("limit must be 1..100 or omitted")
	}
	limit := p.Limit
	if limit == 0 {
		limit = 25
	}
	ids := []string{}
	for id, session := range s.data.Sessions {
		if actor != "operator" && id != actor || p.SessionID != "" && id != p.SessionID || p.Target != "" && session.Target != p.Target {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	start := 0
	if p.Cursor != "" {
		index := sort.SearchStrings(ids, p.Cursor)
		if index == len(ids) || ids[index] != p.Cursor {
			return nil, errors.New("cursor not available in this scope")
		}
		start = index + 1
	}
	end := min(start+limit, len(ids))
	page := RuntimeStatusPage{Supervisor: s.supervisorHealth(now), Build: s.build, Sessions: []RuntimeQueueStatus{}, Notice: "Supervisor and durable queue observations only; provider availability and production readiness are not verified. Counts include retained work, not message contents."}
	for _, id := range ids[start:end] {
		page.Sessions = append(page.Sessions, s.runtimeQueue(s.data.Sessions[id]))
	}
	if end < len(ids) {
		page.NextCursor = ids[end-1]
	}
	return page, nil
}

func (s *Service) runtimeQueue(session Session) RuntimeQueueStatus {
	status := RuntimeQueueStatus{Identity: session.ID, Runtime: session.Runtime, Mode: session.Mode, Busy: session.Busy, NativeBound: session.RuntimeSessionID != ""}
	for _, d := range s.data.Deliveries {
		if d.To != session.ID {
			continue
		}
		switch d.Status {
		case "pending":
			if !s.deliveryRunnable(d) {
				status.WaitingTeam++
			} else if d.Kind == "task" && session.Policy != "workflow" {
				status.BlockedPolicy++
			} else if session.Mode == "managed" {
				status.Ready++
			} else {
				status.ManualPending++
			}
		case "running":
			status.Running++
		case "interrupted":
			status.Interrupted++
		case "failed":
			status.Failed++
			if d.FailureKind == "canceled" {
				status.Canceled++
			}
		case "acknowledged":
			status.Acknowledged++
		case "completed":
			status.Completed++
		}
	}
	return status
}
