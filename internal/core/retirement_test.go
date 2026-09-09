// SPDX-License-Identifier: MPL-2.0

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// retireFixture returns a service whose target is already canonical, plus an
// enrolled manual identity of the shape the operator's registry actually holds:
// a project + name + role identity created by `init`, with an onboarding inbox.
func retireFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	s, dir, target := setup(t)
	canonical, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir, canonical
}

func enrollAgent(t *testing.T, s *Service, id, name, role, target string) (Session, string) {
	t.Helper()
	result := rpc(t, s, "operator", "sessions.enroll", Session{
		ID: id, Name: name, Role: role, Target: target,
		Runtime: "manual", Mode: "manual", Policy: "coordination",
	}).(map[string]any)
	return result["session"].(Session), result["token"].(string)
}

func retireArgs(id string) map[string]any {
	return map[string]any{"id": id, "evidence": "One afternoon's communications test; the identity will never work again."}
}

// The credential is the whole enforcement surface, so retirement has to remove
// it -- and the record has to stay, because the record is what keeps every
// reference to this identity resolvable.
func TestRetireDestroysTheCredentialAndKeepsTheRecord(t *testing.T) {
	s, _, target := retireFixture(t)
	before, token := enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	if actor, ok := s.Authenticate(token); !ok || actor != "agent-alpha" {
		t.Fatalf("fixture credential does not authenticate: %q %v", actor, ok)
	}

	after := rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha")).(Session)

	if after.RetiredAt == "" {
		t.Fatal("retirement recorded no timestamp")
	}
	if _, err := time.Parse(time.RFC3339Nano, after.RetiredAt); err != nil {
		t.Fatalf("retiredAt %q is not RFC3339Nano: %v", after.RetiredAt, err)
	}
	if after.RetiredReason != retireArgs("agent-alpha")["evidence"] {
		t.Fatalf("retirement reason is %q", after.RetiredReason)
	}
	if after.Name != before.Name || after.Role != before.Role || after.Target != before.Target || after.ID != before.ID {
		t.Fatalf("retirement altered the attribution the record exists to keep: %+v -> %+v", before, after)
	}
	if _, ok := s.data.Sessions["agent-alpha"]; !ok {
		t.Fatal("retirement deleted the session record")
	}
	if _, ok := s.data.Tokens["agent-alpha"]; ok {
		t.Fatal("retired identity still has a token in the registry")
	}
	if actor, ok := s.Authenticate(token); ok {
		t.Fatalf("destroyed credential still authenticates as %q", actor)
	}
	// Authenticate and Call are separate gates; the withdrawal has to hold at
	// the one an already-connected caller would reach.
	raw, _ := json.Marshal(map[string]any{})
	if _, err := s.Call("agent-alpha", "sessions.list", raw); err == nil {
		t.Fatal("retired identity still passed the Call authorization gate")
	} else if !strings.Contains(err.Error(), "unauthorized actor") {
		t.Fatalf("Call refused a retired identity for the wrong reason: %q", err)
	}
}

// Excluded by default is the operator's actual pain. Reachable on request is
// what stops the tombstone from destroying the audit trail by omission.
func TestRetiredIdentityIsExcludedFromListingsUnlessAsked(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))

	listed := func(actor string, params map[string]any) map[string]bool {
		t.Helper()
		found := map[string]bool{}
		for _, v := range rpc(t, s, actor, "sessions.list", params).([]Session) {
			found[v.ID] = true
		}
		return found
	}

	if listed("operator", map[string]any{})["agent-alpha"] {
		t.Fatal("retired identity is still in the default operator listing")
	}
	if listed("lead", map[string]any{})["agent-alpha"] {
		t.Fatal("retired identity is still in the default peer listing")
	}
	if !listed("operator", map[string]any{})["builder"] {
		t.Fatal("default listing dropped a live identity too; the filter is wrong")
	}
	withRetired := listed("operator", map[string]any{"includeRetired": true})
	if !withRetired["agent-alpha"] {
		t.Fatal("includeRetired did not return the retired identity")
	}
	if !withRetired["builder"] {
		t.Fatal("includeRetired dropped the live identities")
	}
	denied(t, s, "lead", "sessions.list", map[string]any{"includeRetired": true})
	// The refusal must be the flag, not the listing: the peer still lists.
	rpc(t, s, "lead", "sessions.list", map[string]any{})
}

// This is the review-bypass boundary. A reviewer who dislikes a result must not
// be able to leave, and an operator must not be able to dissolve the obligation
// on their behalf, because either one turns withdrawal into a verdict.
func TestRetirementCannotDissolveAReviewObligation(t *testing.T) {
	s, _, _ := retireFixture(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "redTeam": "red", "idempotencyKey": "bypass"}).(Task)
	claimed := claim(t, s, "builder")
	if err := s.Finish(claimed.ID, "", "Result", nil); err != nil {
		t.Fatal(err)
	}
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "rejected", "evidence": "Not adequate", "expectedRevision": 1})

	for _, participant := range []string{"reviewer", "builder", "lead", "red"} {
		raw, _ := json.Marshal(retireArgs(participant))
		_, err := s.Call("operator", "sessions.retire", raw)
		if err == nil {
			t.Fatalf("retiring %s dissolved a live review obligation", participant)
		}
		if !strings.Contains(err.Error(), task.ID) {
			t.Fatalf("refusal for %s does not name the blocking task %s: %q", participant, task.ID, err)
		}
		if s.data.Sessions[participant].RetiredAt != "" {
			t.Fatalf("%s was retired despite the refusal", participant)
		}
		if _, ok := s.data.Tokens[participant]; !ok {
			t.Fatalf("refused retirement of %s still destroyed its credential", participant)
		}
	}

	// No sequence may reach accepted with the reviewer withdrawn.
	rpc(t, s, "builder", "tasks.submit", map[string]any{"id": task.ID, "output": "Corrected", "expectedRevision": 1})
	denied(t, s, "operator", "sessions.retire", retireArgs("reviewer"))
	for _, actor := range []string{"lead", "operator"} {
		raw, _ := json.Marshal(map[string]any{"id": task.ID})
		_, _ = s.Call(actor, "tasks.accept", raw)
	}
	if got := s.data.Tasks[task.ID].Status; got == "accepted" {
		t.Fatal("task reached accepted without its designated reviewers")
	}

	// The clean path: once the obligation is discharged, withdrawal works.
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Independent evidence", "expectedRevision": 2})
	rpc(t, s, "red", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Adversarial evidence", "expectedRevision": 2})
	accepted := rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID}).(Task)
	if accepted.Status != "accepted" {
		t.Fatalf("clean path did not reach accepted: %q", accepted.Status)
	}
	rpc(t, s, "operator", "sessions.retire", retireArgs("reviewer"))
}

// A verdict is evidence. Withdrawing its author must not touch a byte of it.
func TestRetirementKeepsVerdictsByteIdentical(t *testing.T) {
	s, _, _ := retireFixture(t)
	task := rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "Inspect", "text": "Work", "criteria": "Evidence", "reviewer": "reviewer", "idempotencyKey": "verdicts"}).(Task)
	claimed := claim(t, s, "builder")
	if err := s.Finish(claimed.ID, "", "Result", nil); err != nil {
		t.Fatal(err)
	}
	rpc(t, s, "reviewer", "tasks.review", map[string]any{"id": task.ID, "verdict": "approved", "evidence": "Independent evidence", "expectedRevision": 1})
	rpc(t, s, "lead", "tasks.accept", map[string]any{"id": task.ID})

	before, err := json.Marshal(rpc(t, s, "operator", "tasks.get", map[string]any{"id": task.ID}).(Task).Reviews)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), `"actor":"reviewer"`) {
		t.Fatalf("fixture verdict does not name its actor: %s", before)
	}
	rpc(t, s, "operator", "sessions.retire", retireArgs("reviewer"))
	after, err := json.Marshal(rpc(t, s, "operator", "tasks.get", map[string]any{"id": task.ID}).(Task).Reviews)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("retirement rewrote recorded verdicts: %s -> %s", before, after)
	}
}

// Enrollment resolves server-side and overrides the client's guessed ID, so a
// re-init lands on the retired record. It must refuse, and refusing means all
// three: no adoption, no second identity, no credential.
func TestEnrollDoesNotResurrectARetiredIdentity(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	sessionsBefore := len(s.data.Sessions)
	tokensBefore := len(s.data.Tokens)

	raw, _ := json.Marshal(Session{ID: "agent-guessed-differently", Name: "exp-alpha", Role: "builder",
		Target: target, Runtime: "manual", Mode: "manual", Policy: "coordination"})
	_, err := s.Call("operator", "sessions.enroll", raw)
	if err == nil {
		t.Fatal("enroll resurrected a retired identity")
	}
	for _, want := range []string{"agent-alpha", "retired", "sessions.reinstate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal omits %q: %q", want, err)
		}
	}
	if !strings.Contains(err.Error(), s.data.Sessions["agent-alpha"].RetiredAt) {
		t.Fatalf("refusal omits the retirement timestamp: %q", err)
	}
	if !strings.Contains(err.Error(), s.data.Sessions["agent-alpha"].RetiredReason) {
		t.Fatalf("refusal omits the recorded reason: %q", err)
	}
	if got := len(s.data.Sessions); got != sessionsBefore {
		t.Fatalf("refused enroll minted a second identity: %d -> %d", sessionsBefore, got)
	}
	if _, ok := s.data.Sessions["agent-guessed-differently"]; ok {
		t.Fatal("refused enroll registered the client's guessed ID")
	}
	if got := len(s.data.Tokens); got != tokensBefore {
		t.Fatalf("refused enroll issued a credential: %d -> %d tokens", tokensBefore, got)
	}
	if _, ok := s.data.Tokens["agent-alpha"]; ok {
		t.Fatal("refused enroll restored the retired identity's credential")
	}
	if s.data.Sessions["agent-alpha"].RetiredAt == "" {
		t.Fatal("refused enroll cleared the tombstone")
	}
}

// A retired candidate must not turn a previously unambiguous enroll into an
// ambiguity error, and must not hide a real one either.
func TestEnrollAmbiguityUnchangedByARetiredCandidate(t *testing.T) {
	s, _, target := retireFixture(t)
	// A legacy operator-registered session with no Name is a candidate for any
	// name at this target and role, which is what makes two candidates possible.
	rpc(t, s, "operator", "sessions.register", Session{ID: "legacy-one", Role: "builder", Target: target, Runtime: "manual", Mode: "manual", Policy: "coordination"})
	rpc(t, s, "operator", "sessions.register", Session{ID: "legacy-two", Role: "builder", Target: target, Runtime: "manual", Mode: "manual", Policy: "coordination"})
	raw, _ := json.Marshal(Session{ID: "agent-guess", Name: "exp-alpha", Role: "builder", Target: target, Runtime: "manual", Mode: "manual", Policy: "coordination"})
	if _, err := s.Call("operator", "sessions.enroll", raw); err == nil {
		t.Fatal("two live candidates did not produce an ambiguity error")
	} else if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("two live candidates produced %q, want the ambiguity error", err)
	}

	// Retiring one leaves exactly one live candidate: the enroll must adopt it,
	// not report an ambiguity and not report a retirement.
	rpc(t, s, "operator", "sessions.retire", retireArgs("legacy-two"))
	result, err := s.Call("operator", "sessions.enroll", raw)
	if err != nil {
		t.Fatalf("one live plus one retired candidate was not resolved: %v", err)
	}
	adopted := result.(map[string]any)["session"].(Session)
	if adopted.ID != "legacy-one" {
		t.Fatalf("enroll adopted %q, want the single live candidate legacy-one", adopted.ID)
	}
	if s.data.Sessions["legacy-two"].RetiredAt == "" {
		t.Fatal("the retired candidate was reinstated as a side effect")
	}
}

// Reinstatement is a separate operator act with a new credential; the old one
// is gone and does not come back.
func TestReinstateIssuesANewCredentialAndPreservesTheInbox(t *testing.T) {
	s, _, target := retireFixture(t)
	_, oldToken := enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	send(t, s, "lead", "agent-alpha", "before-retirement")
	rpc(t, s, "operator", "teams.create", map[string]any{"id": "product", "target": target, "title": "Product", "text": "Team brief."})
	rpc(t, s, "operator", "teams.invite", map[string]any{"id": "product", "target": target, "to": "agent-alpha"})
	rpc(t, s, "agent-alpha", "teams.join", map[string]any{"id": "product"})
	first := rpc(t, s, "operator", "inbox.list", map[string]any{"sessionId": "agent-alpha"}).([]Delivery)
	rpc(t, s, "agent-alpha", "inbox.acknowledge", map[string]any{"messageId": first[0].ID})
	inboxBefore := rpc(t, s, "operator", "inbox.list", map[string]any{"sessionId": "agent-alpha"}).([]Delivery)
	if len(inboxBefore) < 3 || !inboxBefore[0].Acknowledged {
		t.Fatalf("fixture inbox is not the one under test: %d messages", len(inboxBefore))
	}
	onboardingBefore := onboardingCount(s, "agent-alpha")

	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	if got := s.data.Teams[target+"\x00"+"product"].Membership["agent-alpha"]; got != "revoked" {
		t.Fatalf("membership after retirement is %q, want revoked", got)
	}

	// The authority check has to be made HERE, while the identity is actually
	// retired. Asserting it after a successful reinstatement would pass on the
	// "identity is not retired" refusal instead, whether or not an operator was
	// ever required.
	denied(t, s, "lead", "sessions.reinstate", map[string]any{"id": "agent-alpha", "evidence": "By a peer."})
	denied(t, s, "builder", "sessions.reinstate", map[string]any{"id": "agent-alpha", "evidence": "By another peer."})
	if s.data.Sessions["agent-alpha"].RetiredAt == "" {
		t.Fatal("a peer reinstated a withdrawn identity")
	}
	if _, ok := s.data.Tokens["agent-alpha"]; ok {
		t.Fatal("a refused peer reinstatement still issued a credential")
	}

	result := rpc(t, s, "operator", "sessions.reinstate", map[string]any{"id": "agent-alpha", "evidence": "The experiment resumed; the operator wants this role back."}).(map[string]any)
	newToken := result["token"].(string)
	restored := result["session"].(Session)

	if newToken == "" || newToken == oldToken {
		t.Fatalf("reinstatement did not issue a new credential (old %q, new %q)", oldToken, newToken)
	}
	if actor, ok := s.Authenticate(oldToken); ok {
		t.Fatalf("destroyed credential works again after reinstatement, as %q", actor)
	}
	if actor, ok := s.Authenticate(newToken); !ok || actor != "agent-alpha" {
		t.Fatalf("new credential does not authenticate: %q %v", actor, ok)
	}
	if restored.RetiredAt != "" || restored.RetiredReason != "" {
		t.Fatalf("reinstatement left the tombstone in place: %+v", restored)
	}
	inboxAfter := rpc(t, s, "operator", "inbox.list", map[string]any{"sessionId": "agent-alpha"}).([]Delivery)
	if len(inboxAfter) != len(inboxBefore) {
		t.Fatalf("inbox size %d -> %d", len(inboxBefore), len(inboxAfter))
	}
	for i := range inboxBefore {
		if inboxAfter[i].ID != inboxBefore[i].ID || inboxAfter[i].Acknowledged != inboxBefore[i].Acknowledged || inboxAfter[i].Handled != inboxBefore[i].Handled {
			t.Fatalf("message %d changed: %+v -> %+v", i, inboxBefore[i], inboxAfter[i])
		}
	}
	if got := onboardingCount(s, "agent-alpha"); got != onboardingBefore {
		t.Fatalf("reinstatement re-sent onboarding: %d -> %d messages", onboardingBefore, got)
	}
	if got := s.data.Teams[target+"\x00"+"product"].Membership["agent-alpha"]; got != "revoked" {
		t.Fatalf("reinstatement restored team membership as %q; re-invitation must stay a separate act", got)
	}
	denied(t, s, "operator", "sessions.reinstate", map[string]any{"id": "agent-alpha", "evidence": "Twice."})
	denied(t, s, "operator", "sessions.reinstate", map[string]any{"id": "agent-alpha", "evidence": " "})
}

func onboardingCount(s *Service, id string) int {
	count := 0
	for _, d := range s.data.Deliveries {
		if d.To == id && d.Provenance == "service-onboarding" {
			count++
		}
	}
	return count
}

// Busy and a live lease are both refusals rather than cleanups: retirement must
// never race a turn in flight or steal a lease another process holds.
func TestRetireRefusesBusyAndLeasedIdentities(t *testing.T) {
	s, _, target := retireFixture(t)
	send(t, s, "lead", "builder", "busy-work")
	claim(t, s, "builder")
	if !s.data.Sessions["builder"].Busy {
		t.Fatal("fixture session is not busy")
	}
	raw, _ := json.Marshal(retireArgs("builder"))
	if _, err := s.Call("operator", "sessions.retire", raw); err == nil {
		t.Fatal("busy identity retired")
	} else if !strings.Contains(err.Error(), "busy") {
		t.Fatalf("busy refusal misdescribes its condition: %q", err)
	}

	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	attachment := rpc(t, s, "agent-alpha", "sessions.attach", map[string]any{"nativeId": "native-one", "runtime": "claude", "target": target}).(Attachment)
	if attachment.ExpiresAt <= time.Now().Unix() {
		t.Fatal("fixture lease is not live")
	}
	if _, err := s.Call("operator", "sessions.retire", mustJSON(t, retireArgs("agent-alpha"))); err == nil {
		t.Fatal("identity holding a live attachment lease retired")
	} else if !strings.Contains(err.Error(), "attachment") {
		t.Fatalf("lease refusal misdescribes its condition: %q", err)
	}
	if s.data.Sessions["agent-alpha"].Attachment.LeaseID == "" {
		t.Fatal("refused retirement cleaned up the lease it declined to take")
	}

	// Expiry, not force, is the remedy.
	expired := s.data.Sessions["agent-alpha"]
	expired.Attachment.ExpiresAt = time.Now().Unix() - 1
	s.data.Sessions["agent-alpha"] = expired
	retired := rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha")).(Session)
	if retired.Attachment != (Attachment{}) {
		t.Fatalf("retirement left a non-terminal attachment: %+v", retired.Attachment)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Delivered is not read. Undelivered work addressed to a withdrawn identity is
// interrupted, never completed, and retry must not resurrect it.
func TestRetireInterruptsUndeliveredWorkWithoutCompletingIt(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	pending := send(t, s, "lead", "agent-alpha", "pending-one")
	acknowledged := send(t, s, "lead", "agent-alpha", "acknowledged-one")
	rpc(t, s, "agent-alpha", "inbox.acknowledge", map[string]any{"messageId": acknowledged.ID})
	if deliveryByID(s, acknowledged.ID).Status != "acknowledged" {
		t.Fatalf("fixture acknowledged delivery is %q", deliveryByID(s, acknowledged.ID).Status)
	}

	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))

	for _, id := range []string{pending.ID, acknowledged.ID} {
		d := deliveryByID(s, id)
		if d.Status != "interrupted" {
			t.Fatalf("undelivered work is %q, want interrupted", d.Status)
		}
		if d.Error != "recipient identity retired" {
			t.Fatalf("interruption reason is %q", d.Error)
		}
		if d.Output != "" {
			t.Fatalf("retirement recorded output %q for work never done", d.Output)
		}
		denied(t, s, "operator", "messages.retry", map[string]any{"messageId": id, "acknowledgeDuplicateRisk": true})
		if got := deliveryByID(s, id).Status; got != "interrupted" {
			t.Fatalf("refused retry still moved the delivery to %q", got)
		}
	}
	// The acknowledgement flag itself is preserved; only the queue state moved.
	if !deliveryByID(s, acknowledged.ID).Acknowledged {
		t.Fatal("retirement cleared a read acknowledgement")
	}
}

func deliveryByID(s *Service, id string) Delivery {
	for _, d := range s.data.Deliveries {
		if d.ID == id {
			return d
		}
	}
	return Delivery{}
}

// The supervisor must not spend a runtime turn on a withdrawn identity.
//
// The pending delivery is restored by hand after retirement on purpose.
// Retirement interrupts undelivered work, so an empty queue would make Claim
// return nil whether or not the retirement guard exists, and the assertion
// would pass on the wrong reason.
func TestClaimYieldsNothingForARetiredManagedSession(t *testing.T) {
	s, _, _ := retireFixture(t)
	queued := send(t, s, "lead", "builder", "queued")
	claimed := claim(t, s, "builder")
	if claimed.ID != queued.ID {
		t.Fatalf("fixture claimed %q, want %q", claimed.ID, queued.ID)
	}
	if err := s.Finish(claimed.ID, "", "done", nil); err != nil {
		t.Fatal(err)
	}

	rpc(t, s, "operator", "sessions.retire", retireArgs("builder"))
	for i := range s.data.Deliveries {
		if s.data.Deliveries[i].ID == queued.ID {
			s.data.Deliveries[i].Status = "pending"
		}
	}
	if d, err := s.Claim("builder"); err != nil || d != nil {
		t.Fatalf("retired managed session yielded work: %v %+v", err, d)
	}
	if got := deliveryByID(s, queued.ID).Status; got != "pending" {
		t.Fatalf("refused claim altered the delivery to %q", got)
	}
}

// A withdrawn identity cannot be given new obligations.
func TestMessagesAndAssignmentsRefuseARetiredIdentity(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.policy", map[string]any{"id": "agent-alpha", "policy": "workflow"})
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))

	denied(t, s, "lead", "messages.send", map[string]any{"to": "agent-alpha", "text": "hello", "idempotencyKey": "after-one"})
	denied(t, s, "operator", "messages.send", map[string]any{"to": "agent-alpha", "text": "hello", "idempotencyKey": "after-two"})
	denied(t, s, "lead", "tasks.assign", map[string]any{"to": "agent-alpha", "title": "T", "text": "W", "criteria": "C", "reviewer": "reviewer", "idempotencyKey": "author-retired"})
	denied(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "T", "text": "W", "criteria": "C", "reviewer": "agent-alpha", "idempotencyKey": "reviewer-retired"})
	denied(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "T", "text": "W", "criteria": "C", "reviewer": "reviewer", "redTeam": "agent-alpha", "idempotencyKey": "red-retired"})
	// The same assignment without the retired identity must still work, or the
	// refusals above prove nothing about which condition stopped them.
	rpc(t, s, "lead", "tasks.assign", map[string]any{"to": "builder", "title": "T", "text": "W", "criteria": "C", "reviewer": "reviewer", "redTeam": "red", "idempotencyKey": "control"})
}

// Board attribution is a bare identity string with no denormalized name, so the
// record has to outlive the credential for the post to stay resolvable.
func TestRetirementKeepsBoardAttribution(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	post := rpc(t, s, "agent-alpha", "board.post", map[string]any{"topic": "technique", "title": "Finding", "text": "Body", "evidence": "live probe", "idempotencyKey": "board-one"}).(BoardPost)

	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))

	after := rpc(t, s, "operator", "board.get", map[string]any{"id": post.ID, "target": target}).(BoardPost)
	if after.Author != "agent-alpha" {
		t.Fatalf("board author is %q after retirement", after.Author)
	}
	if after != post {
		t.Fatalf("retirement altered an immutable post: %+v -> %+v", post, after)
	}
	author, ok := s.data.Sessions[after.Author]
	if !ok || author.Name != "exp-alpha" || author.Role != "builder" {
		t.Fatalf("post author no longer resolves to a named identity: %+v %v", author, ok)
	}
}

// Credential destruction has to be durable state, not merely a code path.
func TestRetirementSurvivesRestartAndRefusesCorruptState(t *testing.T) {
	s, dir, target := retireFixture(t)
	_, token := enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	before := s.data.Sessions["agent-alpha"]
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	after := reopened.data.Sessions["agent-alpha"]
	if after.RetiredAt != before.RetiredAt || after.RetiredReason != before.RetiredReason {
		t.Fatalf("restart altered the tombstone: %+v -> %+v", before, after)
	}
	if _, ok := reopened.data.Tokens["agent-alpha"]; ok {
		t.Fatal("restart restored a credential for a retired identity")
	}
	if _, ok := reopened.Authenticate(token); ok {
		t.Fatal("destroyed credential authenticates after restart")
	}

	// A retired record holding a live token is corruption, not a state to run on.
	reopened.data.Tokens["agent-alpha"] = randomID()
	if err := reopened.save(); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	refused, err := New(dir)
	if err == nil {
		refused.Close()
		t.Fatal("service loaded a retired identity that still held a credential")
	}
	if !strings.Contains(err.Error(), "retired") {
		t.Fatalf("load refusal does not name the invariant it enforces: %q", err)
	}
}

// A retirement that never happened must leave no trace: not the number, not a
// snapshot. This is the "only on success" half, and nothing else.
func TestRefusedRetirementAdvancesNoSchemaAndWritesNoSnapshot(t *testing.T) {
	s, dir, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	if s.data.SchemaVersion != 1 {
		t.Fatalf("fixture schema is %d; adjust this test to its real starting point", s.data.SchemaVersion)
	}
	denied(t, s, "lead", "sessions.retire", retireArgs("agent-alpha"))
	denied(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": " "})
	denied(t, s, "operator", "sessions.retire", retireArgs("no-such-identity"))
	if s.data.SchemaVersion != 1 {
		t.Fatalf("refused retirement advanced the schema to %d", s.data.SchemaVersion)
	}
	if backups := retirementBackups(t, dir); len(backups) != 0 {
		t.Fatalf("refused retirement wrote backups: %v", backups)
	}
}

// The first retirement takes the number, and the snapshot it takes first has to
// be a usable downgrade point.
//
// Counting snapshot FILES is not enough and was the blind spot here: a snapshot
// written after the number was raised is a file of exactly the right name
// carrying exactly the state an older binary would refuse. So this opens it and
// reads what is inside.
func TestFirstRetirementTakesSchemaFiveWithAPreMigrationSnapshot(t *testing.T) {
	s, dir, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	before := s.data.SchemaVersion

	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))

	if s.data.SchemaVersion != retirementSchema {
		t.Fatalf("schema is %d after retirement, want %d", s.data.SchemaVersion, retirementSchema)
	}
	backups := retirementBackups(t, dir)
	if len(backups) != 1 {
		t.Fatalf("want exactly one pre-retirement snapshot, got %v", backups)
	}
	snapshot := readSchemaSnapshot(t, backups[0])
	if snapshot.SchemaVersion != before {
		t.Fatalf("snapshot records schema %d, want the pre-migration %d; a snapshot taken after the bump is not a downgrade point", snapshot.SchemaVersion, before)
	}
	// A snapshot of the post-retirement state would be equally useless, so the
	// identity inside it must still be the live one.
	if saved := snapshot.Sessions["agent-alpha"]; saved.RetiredAt != "" {
		t.Fatalf("snapshot already contains the retirement it was meant to precede: %+v", saved)
	}
	if snapshot.Tokens["agent-alpha"] == "" {
		t.Fatal("snapshot has already lost the credential it was meant to preserve")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.data.SchemaVersion != retirementSchema {
		t.Fatalf("schema did not survive restart: %d", reopened.data.SchemaVersion)
	}
}

// Lazy means once. A migration that re-runs on every retirement writes a fresh
// snapshot each time, filling the state directory with near-duplicates and
// burying the one snapshot that actually precedes the change.
func TestSecondRetirementTakesNoFurtherSnapshot(t *testing.T) {
	s, dir, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	enrollAgent(t, s, "agent-beta", "exp-beta", "reviewer", target)

	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	first := retirementBackups(t, dir)
	if len(first) != 1 {
		t.Fatalf("first retirement wrote %v", first)
	}
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-beta"))
	second := retirementBackups(t, dir)
	if len(second) != 1 || second[0] != first[0] {
		t.Fatalf("second retirement re-ran the migration: %v -> %v", first, second)
	}
}

// readSchemaSnapshot opens a pre-migration snapshot as the state it is, so a
// test can assert what it contains rather than that it exists.
func readSchemaSnapshot(t *testing.T, path string) state {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot state
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func retirementBackups(t *testing.T, dir string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "pre-session-retirement-schema-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// backupSchema serves three features now, so its failure must name retirement.
func TestRetireBackupFailureNamesRetirement(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	if err := os.Chmod(s.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	_, err := s.Call("operator", "sessions.retire", mustJSON(t, retireArgs("agent-alpha")))
	if chmodErr := os.Chmod(s.dir, 0o700); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	if err == nil {
		t.Fatal("retirement succeeded with an unwritable state directory")
	}
	if !strings.Contains(err.Error(), "pre-session-retirement-schema backup failed") {
		t.Fatalf("backup failure does not name retirement: %q", err)
	}
	if s.data.SchemaVersion != 1 {
		t.Fatalf("failed backup advanced the schema to %d", s.data.SchemaVersion)
	}
	if s.data.Sessions["agent-alpha"].RetiredAt != "" {
		t.Fatal("failed backup still retired the identity")
	}
	if _, ok := s.data.Tokens["agent-alpha"]; !ok {
		t.Fatal("failed backup still destroyed the credential")
	}
}

// A failed save must restore the credential mutate snapshotted, not leave the
// identity withdrawn in memory and live on disk.
func TestRetireRollsBackWhenSaveFails(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	enrollAgent(t, s, "agent-beta", "exp-beta", "reviewer", target)
	// Retire once so the schema is already 5: the failure under test is then the
	// state write itself, not the pre-migration snapshot.
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-alpha"))
	if s.data.SchemaVersion != 5 {
		t.Fatalf("schema is %d; the save path is not the one being tested", s.data.SchemaVersion)
	}
	betaToken, err := s.Token("agent-beta")
	if err != nil {
		t.Fatal(err)
	}
	before := mustJSON(t, s.data)

	if err := os.Chmod(s.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	denied(t, s, "operator", "sessions.retire", retireArgs("agent-beta"))
	if err := os.Chmod(s.dir, 0o700); err != nil {
		t.Fatal(err)
	}

	if after := mustJSON(t, s.data); string(after) != string(before) {
		t.Fatalf("failed save left a half-applied retirement: %s -> %s", before, after)
	}
	restored, err := s.Token("agent-beta")
	if err != nil {
		t.Fatalf("rollback did not restore the credential: %v", err)
	}
	if restored != betaToken {
		t.Fatalf("rollback issued a different credential: %q -> %q", betaToken, restored)
	}
	if actor, ok := s.Authenticate(betaToken); !ok || actor != "agent-beta" {
		t.Fatalf("rolled-back credential does not authenticate: %q %v", actor, ok)
	}
	// The rolled-back service must still be able to retire once saving works.
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-beta"))
}

func TestRetireRequiresBoundedEvidence(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	denied(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha"})
	denied(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": ""})
	denied(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": "   \t\n  "})
	denied(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": strings.Repeat("e", 8193)})
	if s.data.Sessions["agent-alpha"].RetiredAt != "" {
		t.Fatal("refused evidence still retired the identity")
	}
	rpc(t, s, "operator", "sessions.retire", map[string]any{"id": "agent-alpha", "evidence": strings.Repeat("e", 8192)})
}

// An agent may not retire itself or anyone. Self-retirement is review evasion.
func TestRetirementIsOperatorOnly(t *testing.T) {
	s, _, target := retireFixture(t)
	enrollAgent(t, s, "agent-alpha", "exp-alpha", "builder", target)
	denied(t, s, "agent-alpha", "sessions.retire", retireArgs("agent-alpha"))
	denied(t, s, "agent-alpha", "sessions.retire", retireArgs("builder"))
	denied(t, s, "lead", "sessions.retire", retireArgs("agent-alpha"))
	if s.data.Sessions["agent-alpha"].RetiredAt != "" || s.data.Sessions["builder"].RetiredAt != "" {
		t.Fatal("a peer withdrew an identity")
	}
	if _, ok := s.data.Tokens["agent-alpha"]; !ok {
		t.Fatal("a refused peer retirement still destroyed a credential")
	}
}

// Retirement state is server-owned; a client may not present itself as retired.
//
// All THREE fields, because params embeds Session and registration copies it
// wholesale. A guard naming two of a three-field tri-state is not a guard: the
// unnamed field is a durable, operator-facing evidence record whose only
// legitimate author is the service.
func TestRegistrationRefusesClientSuppliedRetirementState(t *testing.T) {
	s, _, target := retireFixture(t)
	forged := func(mutate func(*Session)) Session {
		v := Session{ID: "agent-forged", Name: "exp", Role: "builder", Target: target,
			Runtime: "manual", Mode: "manual", Policy: "coordination"}
		mutate(&v)
		return v
	}
	for _, method := range []string{"sessions.register", "sessions.enroll"} {
		denied(t, s, "operator", method, forged(func(v *Session) {
			v.RetiredAt = time.Now().UTC().Format(time.RFC3339Nano)
		}))
		denied(t, s, "operator", method, forged(func(v *Session) {
			v.RetiredReason = "pre-retired"
		}))
		// An OPEN entry: if accepted, the resulting state cannot be loaded
		// again, so one legal RPC call permanently bricks the state directory.
		denied(t, s, "operator", method, forged(func(v *Session) {
			v.Retirements = []Retirement{{At: "2026-01-01T00:00:00Z", Reason: "Fabricated."}}
		}))
		// A CLOSED entry: well-formed, passes every load validator, and comes
		// back through sessions.list as the service's own evidence.
		denied(t, s, "operator", method, forged(func(v *Session) {
			v.Retirements = []Retirement{{At: "2026-01-01T00:00:00Z", Reason: "Fabricated.",
				ReinstatedAt: "2026-01-02T00:00:00Z", ReinstatedReason: "Also fabricated."}}
		}))
	}
	if _, ok := s.data.Sessions["agent-forged"]; ok {
		t.Fatal("a client-supplied retirement state was registered")
	}
	if _, ok := s.data.Tokens["agent-forged"]; ok {
		t.Fatal("a refused registration still minted a credential")
	}
}

// The consequence, asserted where it actually bites: a refused call must leave
// the state directory loadable. Asserting only that the RPC returned an error
// would miss a guard that refuses after writing.
func TestClientSuppliedRetirementStateCannotBrickTheStateDirectory(t *testing.T) {
	s, dir, target := retireFixture(t)
	// Schema 5 is the state of any deployment that has ever retired anyone.
	enrollAgent(t, s, "agent-real", "exp-real", "builder", target)
	rpc(t, s, "operator", "sessions.retire", retireArgs("agent-real"))
	if s.data.SchemaVersion != retirementSchema {
		t.Fatalf("fixture is at schema %d, not the one under test", s.data.SchemaVersion)
	}
	denied(t, s, "operator", "sessions.register", Session{ID: "agent-poison", Target: target,
		Runtime: "manual", Mode: "manual", Policy: "coordination",
		Retirements: []Retirement{{At: "2026-01-01T00:00:00Z", Reason: "Fabricated."}}})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatalf("a refused registration left the state directory unloadable: %v", err)
	}
	defer reopened.Close()
	if _, ok := reopened.data.Sessions["agent-poison"]; ok {
		t.Fatal("the refused registration was persisted after all")
	}
	// The real retirement is untouched, so the refusal cost no evidence.
	if reopened.data.Sessions["agent-real"].RetiredAt == "" {
		t.Fatal("the refusal disturbed a real retirement")
	}
}
