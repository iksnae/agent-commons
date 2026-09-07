// SPDX-License-Identifier: MPL-2.0

package core

// RuntimeActive is a supervisor check, not an agent-facing authority grant.
func (s *Service) RuntimeActive(deliveryID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	for _, d := range s.data.Deliveries {
		if d.ID == deliveryID {
			session := s.data.Sessions[d.To]
			return d.Status == "running" && session.Mode == "managed" && session.Busy && s.deliveryRunnable(d)
		}
	}
	return false
}
