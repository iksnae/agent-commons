// SPDX-License-Identifier: MPL-2.0

package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Session retirement withdraws an identity without erasing it.
//
// Deleting the Session record would buy nothing the credential deletion does
// not already buy, and would cost attribution. deliveryTarget resolves a
// delivery's project through Sessions[d.To], falling back to Sessions[d.From]
// when the recipient is the operator, so a deleted record yields Target == ""
// and deliveryAccess then refuses every team-scoped delivery ADDRESSED TO this
// identity, plus the ones it sent to the operator. Deliveries it sent to a live
// peer resolve through that peer and survive. The operator's own history would
// disappear from inbox.list as a side effect of the withdrawal.
// BoardPost.Author and Review.Actor are bare identity strings with no
// denormalized name, and a tombstone is what keeps them resolvable.
//
// Everything here is operator-only and lives in internal/core because every
// precondition reads Tasks, Teams, Deliveries and Attachment under the single
// process lock. A predicate computed in another layer would be both a race and
// a second copy of the rule.

// retirementSchema is the durable schema number session retirement takes.
// Numbers are never shared between features: 2 and 3 are the team schemas, 4 is
// task abandonment, and retirement takes 5.
const retirementSchema = 5

// enableSessionRetirement advances the state schema the first time an identity
// is actually retired, taking a pre-migration snapshot first. It is lazy for
// the same reason abandonment's is: a directory that never retires anything
// keeps the older schema and stays readable by an older binary.
//
// The bump is what makes a downgrade safe. An older binary has no notion of
// RetiredAt, so it would restore nothing but would also enforce nothing: it
// would list the identity as active, let tasks.assign name it, and let
// sessions.enroll adopt it. That binary already refuses a schema it does not
// know, so raising the number arms a refusal it shipped with.
//
// Callers must run every validation before this: a refused retirement must
// write no backup and advance no number.
func (s *Service) enableSessionRetirement() error {
	if s.data.SchemaVersion >= retirementSchema {
		return nil
	}
	if err := s.backupSchema(fmt.Sprintf("pre-session-retirement-schema-%d-", s.data.SchemaVersion)); err != nil {
		return err
	}
	s.data.SchemaVersion = retirementSchema
	return nil
}

// retired reports whether an identity has been withdrawn. A missing ID yields
// the zero Session, whose empty RetiredAt is never retired, so a map miss is
// safe and callers that also test existence keep their own error.
func (s *Service) retired(id string) bool {
	return s.data.Sessions[id].RetiredAt != ""
}

// validateRetirement refuses to load state whose retirement records and
// credentials disagree, so that credential destruction is durable state rather
// than only a code path. It mirrors validateTeams: a schema floor for the
// feature's records, then the per-record invariant.
func (s *Service) validateRetirement() error {
	for id, v := range s.data.Sessions {
		if v.RetiredAt == "" && v.RetiredReason == "" && len(v.Retirements) == 0 {
			continue
		}
		if s.data.SchemaVersion < retirementSchema {
			return errors.New("retired identity records require schema 5")
		}
		if err := validateRetirementLedger(v); err != nil {
			return err
		}
		if v.RetiredAt != "" && s.data.Tokens[id] != "" {
			return errors.New("retired identity still holds a credential")
		}
	}
	return nil
}

// validateRetirementLedger makes current state and history agree by checking
// rather than by trusting that both writers stayed correct. RetiredAt and the
// ledger are two representations of one fact, and two representations that are
// never compared drift.
func validateRetirementLedger(v Session) error {
	for _, entry := range v.Retirements {
		if entry.At == "" || strings.TrimSpace(entry.Reason) == "" {
			return errors.New("retirement history entry is missing its timestamp or reason")
		}
		if entry.ReinstatedAt == "" && strings.TrimSpace(entry.ReinstatedReason) != "" {
			return errors.New("retirement history records a reinstatement reason with no timestamp")
		}
	}
	// Every entry but the last must be closed, or "the open one" is ambiguous.
	for _, entry := range v.Retirements[:max(len(v.Retirements)-1, 0)] {
		if entry.ReinstatedAt == "" {
			return errors.New("retirement history has more than one open entry")
		}
	}
	var open *Retirement
	if n := len(v.Retirements); n > 0 && v.Retirements[n-1].ReinstatedAt == "" {
		open = &v.Retirements[n-1]
	}
	if v.RetiredAt == "" {
		if strings.TrimSpace(v.RetiredReason) != "" {
			return errors.New("retirement reason recorded without a timestamp")
		}
		if open != nil {
			return errors.New("live identity has an open retirement history entry")
		}
		return nil
	}
	if strings.TrimSpace(v.RetiredReason) == "" {
		return errors.New("retired identity is missing its reason")
	}
	if open == nil {
		return errors.New("retired identity has no open retirement history entry")
	}
	if open.At != v.RetiredAt || open.Reason != v.RetiredReason {
		return errors.New("retirement history disagrees with the current retirement")
	}
	return nil
}

// sessionView is the ONLY place a stored Session becomes something handed out.
// Two rules live here so neither can be re-applied differently per call site:
// retirement history is operator evidence and is stripped from everyone else,
// and no returned Session ever aliases the slice the registry holds.
//
// The stripping matters specifically because history outlives reinstatement. A
// retired session never reaches a peer -- it is excluded from sessions.list and
// includeRetired is operator-only -- but a REINSTATED one is listed to every
// peer scoped to its target, and it still carries the ledger.
func sessionView(actor string, v Session) Session {
	if actor != "operator" || len(v.Retirements) == 0 {
		v.Retirements = nil
		return v
	}
	v.Retirements = append([]Retirement{}, v.Retirements...)
	return v
}

// retirementRefusal names the condition blocking a withdrawal, or "" when there
// is none. All three refusals are hard and there is no force flag:
// messages.retry has one because a duplicate side effect is genuinely the
// operator's to weigh, and evidence integrity is not.
func (s *Service) retirementRefusal(v Session, now int64) string {
	if v.Busy {
		return "cannot retire a busy identity"
	}
	if v.Attachment.ExpiresAt > now {
		return "cannot retire an identity holding a live attachment lease; wait for it to expire or release it with sessions.detach"
	}
	// Retiring a designated reviewer must never dissolve a review obligation,
	// so participation in any role blocks withdrawal until the task is
	// terminal. Terminality comes from the shared helper rather than a second
	// derivation of it, exactly as sessions.policy's near-identical gate does.
	var blocking []string
	for _, t := range s.data.Tasks {
		if terminalTaskStatus(t.Status) {
			continue
		}
		if t.Lead == v.ID || t.Author == v.ID || t.Reviewer == v.ID || t.RedTeam == v.ID {
			blocking = append(blocking, t.ID)
		}
	}
	if len(blocking) > 0 {
		sort.Strings(blocking)
		return "cannot retire an identity participating in an unresolved task: " + strings.Join(blocking, ", ")
	}
	return ""
}

// retirementEvidence enforces the same discipline tasks.abandon, tasks.review
// and inbox.handle already enforce, at the same bound.
func retirementEvidence(action, evidence string) error {
	if strings.TrimSpace(evidence) == "" || len(evidence) > 8192 {
		return errors.New(action + " evidence (max 8KiB) required")
	}
	return nil
}

// retire tombstones an identity in place. The credential is destroyed, which is
// the whole enforcement surface: Authenticate iterates s.data.Tokens and Call
// rejects any actor absent from that map, so deleting one entry withdraws the
// identity completely.
//
// This is operator-only, and an agent may not retire itself. Mechanically
// self-retirement destroys the caller's own credential mid-call so it could not
// read its own outcome; structurally it is review evasion, because a reviewer
// who dislikes a result would withdraw instead of recording a verdict. An agent
// that wants out sends a message and the operator decides.
func (s *Service) retire(actor string, p params) (any, error) {
	if actor != "operator" {
		return nil, errors.New("operator required")
	}
	if err := retirementEvidence("retirement", p.Evidence); err != nil {
		return nil, err
	}
	v, ok := s.data.Sessions[p.ID]
	if !ok {
		return nil, errors.New("identity unavailable")
	}
	if v.RetiredAt != "" {
		return nil, errors.New("identity is already retired")
	}
	if refusal := s.retirementRefusal(v, time.Now().Unix()); refusal != "" {
		return nil, errors.New(refusal)
	}
	if err := s.enableSessionRetirement(); err != nil {
		return nil, err
	}
	v.RetiredAt = time.Now().UTC().Format(time.RFC3339Nano)
	v.RetiredReason = p.Evidence
	// Append rather than replace: an identity can be withdrawn and returned
	// more than once, and each cycle is its own record.
	v.Retirements = append(append([]Retirement{}, v.Retirements...),
		Retirement{At: v.RetiredAt, Reason: v.RetiredReason})
	// The lease is already expired -- a live one is refused above -- so this
	// writes the terminal value sessions.detach writes, rather than revoking
	// anything a holder still owns.
	v.Attachment = Attachment{}
	s.data.Sessions[v.ID] = v
	delete(s.data.Tokens, v.ID)
	// Undelivered work becomes interrupted, never completed: delivered does not
	// mean read, and marking it completed would claim this identity acted on
	// messages it never saw. Interrupted is the shape the restart path already
	// writes for work that stopped without a result.
	for i := range s.data.Deliveries {
		d := &s.data.Deliveries[i]
		if d.To == v.ID && (d.Status == "pending" || d.Status == "acknowledged") {
			d.Status = "interrupted"
			d.Error = "recipient identity retired"
		}
	}
	// Membership is revoked rather than removed, reusing the value teams.revoke
	// already writes, so the team's record of who was ever in it survives.
	for key, team := range s.data.Teams {
		if _, member := team.Membership[v.ID]; member {
			team.Membership[v.ID] = "revoked"
			s.data.Teams[key] = team
		}
	}
	return sessionView(actor, v), nil
}

// reinstate returns a retired identity to service. It is a separate, explicit
// operator act because sessions.enroll refuses to resurrect: silently adopting
// a retired record would restore a live credential with no operator decision.
//
// The identity is the same and its inbox, board posts and task history come
// back untouched, acknowledgement flags included. The credential does not: the
// old one was destroyed and a new one is issued. Team memberships stay revoked,
// because re-invitation is its own deliberate act.
func (s *Service) reinstate(actor string, p params) (any, error) {
	if actor != "operator" {
		return nil, errors.New("operator required")
	}
	if err := retirementEvidence("reinstatement", p.Evidence); err != nil {
		return nil, err
	}
	v, ok := s.data.Sessions[p.ID]
	if !ok {
		return nil, errors.New("identity unavailable")
	}
	if v.RetiredAt == "" {
		return nil, errors.New("identity is not retired")
	}
	// Close the open ledger entry BEFORE clearing current state, so the
	// evidence this method demanded is recorded rather than demanded and
	// discarded, and so the withdrawal it reverses is still legible afterwards.
	history := append([]Retirement{}, v.Retirements...)
	if n := len(history); n == 0 || history[n-1].ReinstatedAt != "" {
		return nil, errors.New("retired identity has no open retirement history entry")
	} else {
		history[n-1].ReinstatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		history[n-1].ReinstatedReason = p.Evidence
	}
	v.Retirements = history
	v.RetiredAt = ""
	v.RetiredReason = ""
	s.data.Sessions[v.ID] = v
	token := randomID()
	s.data.Tokens[v.ID] = token
	// s.welcome is idempotent on its onboarding key, so nothing re-sends the
	// onboarding messages this identity already read.
	return map[string]any{"session": sessionView(actor, v), "token": token}, nil
}

// retiredEnrollmentRefusal is what sessions.enroll answers when the candidate
// it resolved to is a tombstone. It names the identity, when it was retired and
// why, and both ways forward, because the client's derived ID is only a guess
// and the operator otherwise has no way to see which record was hit.
func retiredEnrollmentRefusal(v Session) error {
	return fmt.Errorf("identity %s for this project and role was retired at %s (%s); reinstate it with sessions.reinstate, or enroll a different name or role",
		v.ID, v.RetiredAt, v.RetiredReason)
}
