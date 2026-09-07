// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"strings"
)

// BindRuntime checkpoints supervisor-owned native preparation before a model
// turn. It is not an agent-facing RPC or permission to adopt an existing session.
func (s *Service) BindRuntime(deliveryID, runtimeID string) error {
	if strings.TrimSpace(runtimeID) != runtimeID || runtimeID == "" || len(runtimeID) > 256 || strings.ContainsRune(runtimeID, 0) {
		return errors.New("exact native runtime identity required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("closed")
	}
	_, err := s.mutate(func() (any, error) {
		for _, delivery := range s.data.Deliveries {
			if delivery.ID != deliveryID {
				continue
			}
			session := s.data.Sessions[delivery.To]
			if delivery.Status != "running" || session.Mode != "managed" || !session.Busy || !s.deliveryRunnable(delivery) {
				return nil, errors.New("runtime binding requires a running managed delivery")
			}
			if session.RuntimeSessionID != "" && session.RuntimeSessionID != runtimeID {
				return nil, errors.New("native runtime already bound; refusing replacement")
			}
			session.RuntimeSessionID = runtimeID
			s.data.Sessions[session.ID] = session
			return nil, nil
		}
		return nil, errors.New("delivery missing")
	})
	return err
}
