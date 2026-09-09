// SPDX-License-Identifier: MPL-2.0

package core

// registeredSession is the identity sessions.register and sessions.enroll are
// allowed to store, built field by named field from what the client sent.
//
// It replaces `v := p.Session`. That copy was the defect, not a shortcut:
// params embeds Session, so one wholesale assignment made client-settable the
// DEFAULT for every field Session would ever gain, and the guards that refuse
// forged retirement and runtime state are subtractions from that default --
// hand-maintained lists that a field added elsewhere does not join. That is not
// hypothetical. Session.Retirements arrived in v0.0.5 from a decision made
// without sight of the guard, and a register call carrying an open ledger entry
// was accepted and persisted, after which the service would not start again and
// no supported operation could repair it.
//
// This is readOnlyMethods (service.go) with the polarity restored: membership
// is the exception, omission is safe, and a new field defaults to correct by
// exclusion. The cost is that a future field is dropped silently rather than
// refused loudly, which is accepted -- params decoding already rejects unknown
// JSON keys, so only a deliberate schema addition reaches here, and discarded
// is recoverable where persisted is not.
//
// Both existing guards stay exactly where they are. They read the client's
// input, not this result, because they are the operator-facing refusals the
// contract promises for state already known to be server-owned; this function
// is what stops the NEXT field from needing them.
//
// Two tests cover this, and they measure different things; be precise about
// which. TestRegistrationPersistsOnlyClientSettableSessionFields walks Session
// by reflection and asserts at PERSISTED state. What that pins is the guards,
// plus the exhaustiveness trigger that fails naming any field the sentinel
// lists have not decided about -- and, for a client-settable field, that this
// list still carries it through. It does NOT pin the omission half. A
// server-owned field silently left at its zero value here is invisible to it,
// because every server-owned field that exists today is independently refused
// by the two guards before anything is written, so persisted state stays
// correct even with this function reverted to the wholesale copy it replaced.
// TestRegisteredSessionKeepsClientFieldsAndDropsServerOwnedOnes closes that
// half by calling this function directly and checking its result field by
// field, where nothing downstream can absorb the difference.
func registeredSession(in Session) Session {
	return Session{
		ID:     in.ID,
		Target: in.Target,
		Name:   in.Name,
		Role:   in.Role,
		Team:   in.Team,
		Policy: in.Policy,
		// Runtime and Mode are client input -- registration validates them as
		// the shape of the identity being registered. The supervisor-owned
		// runtime state is Attachment, RuntimeSessionID and Busy, which are
		// absent here.
		Runtime:      in.Runtime,
		Mode:         in.Mode,
		Instructions: in.Instructions,
	}
}
