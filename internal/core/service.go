// SPDX-License-Identifier: MPL-2.0

package core

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"agentcommons/internal/harness"
)

type state struct {
	Teams         map[string]teamRecord
	SchemaVersion int
	Board         []BoardPost
	Sessions      map[string]Session
	Tokens        map[string]string
	Deliveries    []Delivery
	Tasks         map[string]Task
	Contexts      map[string][]Context
	Keys          map[string]string
}
type Service struct {
	mu         sync.Mutex
	dir        string
	lock       *os.File
	data       state
	closed     bool
	supervisor supervisorObservation
	build      BuildIdentity
}

var validID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:/-]{0,199}$`)

func checkID(id string) bool {
	return validID.MatchString(id) && !strings.Contains(id, "..") && !strings.HasPrefix(id, "/")
}
func randomID() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func New(directory string) (*Service, error) {
	if info, err := os.Lstat(directory); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return nil, errors.New("state directory must be a real directory")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(directory, "lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("lock must be regular file")
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("state already locked: %w", err)
	}
	s := &Service{dir: directory, lock: f, build: newBuildIdentity(time.Now()), data: state{Sessions: map[string]Session{}, Tokens: map[string]string{}, Tasks: map[string]Task{}, Contexts: map[string][]Context{}, Keys: map[string]string{}}}
	loaded, err := readStateFile(filepath.Join(directory, "state.json"))
	existingState := err == nil
	if err == nil {
		s.data = loaded
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	if s.data.SchemaVersion < 0 {
		s.Close()
		return nil, errors.New("negative state schema version")
	}
	if s.data.SchemaVersion > 4 {
		s.Close()
		return nil, errors.New("state schema newer than this binary")
	}
	if s.data.SchemaVersion >= 2 && s.data.Teams == nil {
		s.Close()
		return nil, errors.New("team state missing from team-aware schema")
	}
	if s.data.Teams == nil {
		s.data.Teams = map[string]teamRecord{}
	}
	if err = s.validateTeams(); err != nil {
		s.Close()
		return nil, err
	}
	if err = s.validateTeamWork(); err != nil {
		s.Close()
		return nil, err
	}
	if s.data.SchemaVersion == 0 {
		s.data.SchemaVersion = 1
	}
	if s.data.Sessions == nil || s.data.Tokens == nil || s.data.Tasks == nil || s.data.Contexts == nil || s.data.Keys == nil {
		s.Close()
		return nil, errors.New("invalid state")
	}
	if s.data.Tokens["operator"] == "" {
		if existingState {
			s.Close()
			return nil, errors.New("existing state is missing its operator credential; not regenerated")
		}
		s.data.Tokens["operator"] = randomID()
	}
	for i := range s.data.Deliveries {
		d := &s.data.Deliveries[i]
		if d.Provenance == "" {
			d.Provenance = "legacy-unverified"
		}
		if d.ThreadID == "" {
			d.ThreadID = d.ID
		}
		if d.Status == "running" {
			d.Status = "interrupted"
			d.Error = "supervisor stopped before completion"
			if d.Kind == "task" && s.data.Tasks[d.TaskID].Status == "working" {
				t := s.data.Tasks[d.TaskID]
				t.Status = "interrupted"
				s.data.Tasks[t.ID] = t
			}
		}
	}
	for id, v := range s.data.Sessions {
		if v.Policy == "" {
			v.Policy = "coordination"
		}
		v.Busy = false
		s.data.Sessions[id] = v
	}
	if err = s.save(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Service) save() error {
	b, err := encodeState(s.data)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), filepath.Join(s.dir, "state.json")); err != nil {
		return err
	}
	d, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (s *Service) mutate(fn func() (any, error)) (any, error) {
	b, _ := json.Marshal(s.data)
	v, err := fn()
	if err == nil {
		err = s.save()
	}
	if err != nil {
		var previous state
		_ = json.Unmarshal(b, &previous)
		s.data = previous
		return nil, err
	}
	return v, nil
}
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	return s.lock.Close()
}
func (s *Service) Authenticate(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", false
	}
	for a, t := range s.data.Tokens {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			return a, true
		}
	}
	return "", false
}
func (s *Service) Token(actor string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("closed")
	}
	t, ok := s.data.Tokens[actor]
	if !ok {
		return "", errors.New("unknown actor")
	}
	return t, nil
}
func (s *Service) Sessions() []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Session{}
	for _, v := range s.data.Sessions {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type params struct {
	Session
	TeamID                   string `json:"teamId"`
	NativeID                 string `json:"nativeId"`
	LeaseID                  string `json:"leaseId"`
	Epoch                    uint64 `json:"epoch"`
	Topic                    string `json:"topic"`
	Query                    string `json:"query"`
	Order                    string `json:"order"`
	Provenance               string `json:"provenance"`
	ReplyTo                  string `json:"replyTo"`
	Cursor                   string `json:"cursor"`
	Limit                    int    `json:"limit"`
	UnreadOnly               bool   `json:"unreadOnly"`
	UnhandledOnly            bool   `json:"unhandledOnly"`
	To                       string `json:"to"`
	Text                     string `json:"text"`
	IdempotencyKey           string `json:"idempotencyKey"`
	ContextID                string `json:"contextId"`
	ContextVersion           int    `json:"contextVersion"`
	SessionID                string `json:"sessionId"`
	MessageID                string `json:"messageId"`
	ExpectedVersion          int    `json:"expectedVersion"`
	Version                  int    `json:"version"`
	Title                    string `json:"title"`
	Criteria                 string `json:"criteria"`
	Reviewer                 string `json:"reviewer"`
	RedTeam                  string `json:"redTeam"`
	Verdict                  string `json:"verdict"`
	Evidence                 string `json:"evidence"`
	Output                   string `json:"output"`
	ExpectedRevision         int    `json:"expectedRevision"`
	AcknowledgeDuplicateRisk bool   `json:"acknowledgeDuplicateRisk"`
}

func (s *Service) Call(actor, method string, raw json.RawMessage) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("closed")
	}
	if _, ok := s.data.Tokens[actor]; !ok {
		return nil, errors.New("unauthorized actor")
	}
	if actor != "operator" && s.data.Sessions[actor].Policy == "coordination" && (strings.HasPrefix(method, "tasks.") && method != "tasks.get" && method != "tasks.list" || method == "context.put") {
		return nil, errors.New("identity policy forbids this operation")
	}
	var p params
	if len(raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&p); err != nil {
			return nil, err
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, errors.New("one JSON object required")
		}
	}
	switch method {
	case "runtime.status", "teams.list", "teams.get", "board.list", "board.get", "sessions.list", "sessions.capabilities", "inbox.page", "inbox.list", "context.get", "tasks.get", "tasks.list":
		return s.call(actor, method, p)
	default:
		return s.mutate(func() (any, error) { return s.call(actor, method, p) })
	}
}
func (s *Service) scoped(actor, target string) bool {
	return actor == "operator" || s.data.Sessions[actor].Target == target
}
func (s *Service) target(actor, requested string) (string, error) {
	if actor != "operator" {
		t := s.data.Sessions[actor].Target
		if requested != "" && requested != t {
			return "", errors.New("target forbidden")
		}
		return t, nil
	}
	return canonicalTarget(requested)
}
func canonicalTarget(t string) (string, error) {
	if !filepath.IsAbs(t) {
		return "", errors.New("absolute target required")
	}
	t, err := filepath.EvalSymlinks(t)
	if err != nil {
		return "", err
	}
	v, err := os.Stat(t)
	if err != nil {
		return "", err
	}
	if !v.IsDir() {
		return "", errors.New("target is not a directory")
	}
	return t, nil
}
func (s *Service) enqueue(from, to, text, kind, task, cid string, version int) Delivery {
	d := Delivery{ID: randomID(), From: from, To: to, Text: text, Kind: kind, TaskID: task, ContextID: cid, ContextVersion: version, Status: "pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	d.Provenance = "peer-assertion"
	if from == "operator" {
		d.Provenance = "operator-credential"
	}
	d.ThreadID = d.ID
	s.data.Deliveries = append(s.data.Deliveries, d)
	return d
}
func (s *Service) pinned(target string, p params) (string, int, error) {
	if p.ContextID == "" {
		if p.ContextVersion != 0 {
			return "", 0, errors.New("context ID required")
		}
		return p.Text, 0, nil
	}
	vs := s.data.Contexts[contextKey(target, p.TeamID, p.ContextID)]
	v := p.ContextVersion
	if v == 0 {
		v = len(vs)
	}
	if v < 1 || v > len(vs) {
		return "", 0, errors.New("context version missing")
	}
	return p.Text + "\n\nShared context (" + p.ContextID + fmt.Sprintf(" v%d):\n", v) + vs[v-1].Text, v, nil
}
func (s *Service) call(actor, method string, p params) (any, error) {
	fail := func(msg string) (any, error) { return nil, errors.New(msg) }
	if p.TeamID != "" && method != "messages.send" && method != "tasks.assign" && method != "context.put" && method != "context.get" {
		return fail("teamId is only valid for new work and context access")
	}
	if strings.HasPrefix(method, "teams.") {
		return s.teams(actor, method, p)
	}
	if strings.HasPrefix(method, "board.") {
		return s.board(actor, method, p)
	}
	if method == "tasks.abandon" {
		return s.abandon(actor, p)
	}
	switch method {
	case "runtime.status":
		return s.runtimeStatus(actor, p, time.Now())
	case "sessions.attach", "sessions.renew", "sessions.detach", "sessions.abort":
		return s.attach(actor, method, p)
	case "sessions.capabilities":
		return map[string]any{"identity": actor, "policy": s.data.Sessions[actor].Policy, "target": s.data.Sessions[actor].Target, "runtimeStatusAvailable": true, "teamMembershipAvailable": true, "teamWorkAvailable": true, "boardOrderingAvailable": true, "messageGrantsAuthority": false, "externalProcessEnforcement": false, "repositoryWriteGranted": false, "deploymentGranted": false, "registrationOperatorOnly": true}, nil
	case "sessions.policy":
		if actor != "operator" {
			return fail("operator required")
		}
		v, ok := s.data.Sessions[p.ID]
		if !ok || (p.Policy != "coordination" && p.Policy != "workflow") {
			return fail("existing identity and valid policy required")
		}
		if v.Busy {
			return fail("cannot change busy identity policy")
		}
		if p.Policy == "coordination" {
			for _, t := range s.data.Tasks {
				if !terminalTaskStatus(t.Status) && (t.Lead == v.ID || t.Author == v.ID || t.Reviewer == v.ID || t.RedTeam == v.ID) {
					return fail("cannot restrict an identity participating in an unresolved task")
				}
			}
		}
		v.Policy = p.Policy
		s.data.Sessions[v.ID] = v
		return v, nil
	case "sessions.register", "sessions.enroll":
		if actor != "operator" {
			return fail("operator required")
		}
		v := p.Session
		if v.Policy == "" {
			v.Policy = "coordination"
		}
		if v.Policy != "coordination" && v.Policy != "workflow" {
			return fail("invalid policy")
		}
		if !checkID(v.ID) || v.ID == "operator" {
			return fail("invalid session ID")
		}
		if method == "sessions.enroll" {
			// Resolve server-side, before ID equality is checked: a legacy
			// operator-registered session (Name == "") or an already-named
			// session for this exact target+role never matches the client's
			// freshly derived ID, so without this lookup enroll would mint a
			// second identity for a role that already exists. Route a match
			// into the adopt branch below by reusing its existing ID, so the
			// existing token is preserved rather than reissued.
			target, err := canonicalTarget(v.Target)
			if err != nil {
				return nil, err
			}
			var candidates []string
			for id, other := range s.data.Sessions {
				if other.Target == target && other.Role == v.Role && (other.Name == "" || other.Name == v.Name) {
					candidates = append(candidates, id)
				}
			}
			switch len(candidates) {
			case 0:
				// no existing match; enroll proceeds to mint a new identity below
			case 1:
				v.ID = candidates[0]
			default:
				sort.Strings(candidates)
				return fail(fmt.Sprintf("ambiguous target+role identity for enroll: %d candidates (%s); supply --id to select one", len(candidates), strings.Join(candidates, ", ")))
			}
		}
		if _, ok := s.data.Sessions[v.ID]; ok {
			if method == "sessions.enroll" {
				existing := s.data.Sessions[v.ID]
				target, err := canonicalTarget(v.Target)
				if err != nil {
					return nil, err
				}
				if existing.Target != target || (existing.Name != "" && existing.Name != v.Name) || existing.Role != v.Role || existing.Team != v.Team || existing.Mode != "manual" || v.Mode != "manual" || existing.Policy != v.Policy {
					return fail("enrollment conflicts with existing identity")
				}
				if existing.Name == "" && v.Name != "" {
					for _, other := range s.data.Sessions {
						if other.ID != existing.ID && other.Target == target && other.Name == v.Name && other.Role == v.Role {
							return fail("project/name/role identity already exists")
						}
					}
					existing.Name = v.Name
					s.data.Sessions[existing.ID] = existing
				}
				s.welcome(existing)
				return map[string]any{"session": existing, "token": s.data.Tokens[v.ID]}, nil
			}
			return fail("session already registered")
		}
		if !harness.CanAttach(v.Runtime) && v.Runtime != "manual" {
			return fail("invalid runtime")
		}
		if v.Mode != "managed" && v.Mode != "manual" {
			return fail("invalid mode")
		}
		if v.Mode == "managed" && !harness.CanManage(v.Runtime) {
			return fail("managed dispatch adapter unavailable for this runtime; use explicit manual coordination")
		}
		if v.RuntimeSessionID != "" || v.Busy || v.Attachment != (Attachment{}) {
			return fail("runtime state is supervisor owned")
		}
		t, err := canonicalTarget(v.Target)
		if err != nil {
			return nil, err
		}
		v.Target = t
		if v.Name != "" {
			for _, existing := range s.data.Sessions {
				if existing.Target == t && existing.Name == v.Name && existing.Role == v.Role {
					return fail("project/name/role identity already exists")
				}
			}
		}
		s.data.Sessions[v.ID] = v
		tok := randomID()
		s.data.Tokens[v.ID] = tok
		if method == "sessions.enroll" {
			s.welcome(v)
		}
		return map[string]any{"session": v, "token": tok}, nil
	case "sessions.list":
		out := []Session{}
		for _, v := range s.data.Sessions {
			if s.scoped(actor, v.Target) {
				out = append(out, v)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	case "messages.send", "tasks.assign":
		if len(p.Text) > 128<<10 {
			return fail("message text exceeds 128KiB")
		}
		if p.Provenance != "" && p.Provenance != "peer-assertion" && p.Provenance != "peer-relayed" {
			return fail("provenance cannot assert operator authority")
		}
		var parent *Delivery
		if p.ReplyTo != "" {
			for _, d := range s.data.Deliveries {
				if d.ID == p.ReplyTo && d.To == actor && d.From == p.To {
					copy := d
					parent = &copy
					break
				}
			}
			if parent == nil {
				return fail("reply must address the sender of a message received by this identity")
			}
		}
		to, ok := s.data.Sessions[p.To]
		if !ok || !s.scoped(actor, to.Target) {
			return fail("recipient unavailable")
		}
		if !s.teamAccess(actor, to.Target, p.TeamID) || !s.teamAccess(to.ID, to.Target, p.TeamID) {
			return fail("joined team participants required")
		}
		if parent != nil && (parent.TeamID != p.TeamID || !s.deliveryAccess(actor, *parent)) {
			return fail("reply must retain accessible team scope")
		}
		if strings.TrimSpace(p.Text) == "" || p.IdempotencyKey == "" {
			return fail("text and idempotencyKey required")
		}
		key := actor + "\x00" + method + "\x00" + p.IdempotencyKey
		if id, ok := s.data.Keys[key]; ok {
			if method == "tasks.assign" {
				existing := s.data.Tasks[id]
				if existing.TeamID != p.TeamID || !s.teamAccess(actor, existing.Target, existing.TeamID) {
					return fail("idempotency key belongs to different or unavailable scope")
				}
				return cloneTask(existing), nil
			}
			for _, d := range s.data.Deliveries {
				if d.ID == id {
					if d.TeamID != p.TeamID || !s.deliveryAccess(actor, d) {
						return fail("idempotency key belongs to different or unavailable scope")
					}
					return d, nil
				}
			}
		}
		txt, ver, err := s.pinned(to.Target, p)
		if err != nil {
			return nil, err
		}
		if len(txt) > 128<<10 {
			return fail("pinned message exceeds 128KiB")
		}
		if method == "messages.send" {
			if err := s.enableTeamWork(p.TeamID); err != nil {
				return nil, err
			}
			d := s.enqueue(actor, p.To, txt, "message", "", p.ContextID, ver)
			d.TeamID = p.TeamID
			if actor != "operator" && p.Provenance != "" {
				d.Provenance = p.Provenance
			}
			if parent != nil {
				d.ReplyTo = parent.ID
				d.ThreadID = parent.ThreadID
			}
			s.data.Deliveries[len(s.data.Deliveries)-1] = d
			s.data.Keys[key] = d.ID
			return d, nil
		}
		if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Criteria) == "" {
			return fail("title and criteria required")
		}
		if len(p.Criteria) > 16<<10 || len(p.Title) > 4096 {
			return fail("task title or criteria too large")
		}
		if to.Policy != "workflow" {
			return fail("task author requires workflow policy")
		}
		r, ok := s.data.Sessions[p.Reviewer]
		if !ok || r.Target != to.Target || r.ID == to.ID || r.Policy != "workflow" {
			return fail("independent reviewer required")
		}
		if p.RedTeam != "" {
			r, ok := s.data.Sessions[p.RedTeam]
			if !ok || r.Target != to.Target || r.ID == to.ID || r.ID == p.Reviewer || r.Policy != "workflow" {
				return fail("independent red team required")
			}
		}
		if !s.teamAccess(p.Reviewer, to.Target, p.TeamID) || p.RedTeam != "" && !s.teamAccess(p.RedTeam, to.Target, p.TeamID) {
			return fail("review participants must join the task team")
		}
		if err := s.enableTeamWork(p.TeamID); err != nil {
			return nil, err
		}
		t := Task{TeamID: p.TeamID, ID: randomID(), Target: to.Target, Lead: actor, Author: to.ID, Title: p.Title, Criteria: p.Criteria, Reviewer: p.Reviewer, RedTeam: p.RedTeam, Status: "assigned", Reviews: []Review{}}
		s.data.Tasks[t.ID] = t
		s.data.Keys[key] = t.ID
		s.enqueue(actor, to.ID, txt+"\n\nAcceptance criteria: "+p.Criteria, "task", t.ID, p.ContextID, ver)
		s.data.Deliveries[len(s.data.Deliveries)-1].TeamID = p.TeamID
		return cloneTask(t), nil
	case "inbox.list", "inbox.page":
		id := actor
		if actor == "operator" && p.SessionID != "" {
			id = p.SessionID
		} else if p.SessionID != "" && p.SessionID != actor {
			return fail("inbox forbidden")
		}
		out := []Delivery{}
		start := p.Cursor == ""
		if p.Limit < 0 || p.Limit > 100 {
			return fail("limit must be 1..100 or omitted")
		}
		limit := p.Limit
		if limit == 0 {
			limit = 50
		}
		next := ""
		size := 0
		for _, d := range s.data.Deliveries {
			if d.To == id && s.deliveryAccess(actor, d) {
				if !start {
					if d.ID == p.Cursor {
						start = true
					}
					continue
				}
				if p.UnreadOnly && d.Acknowledged || p.UnhandledOnly && d.Handled {
					continue
				}
				b, _ := json.Marshal(d)
				if method == "inbox.page" && len(out) > 0 && (len(out) >= limit || size+len(b) > 512<<10) {
					next = out[len(out)-1].ID
					break
				}
				size += len(b)
				if method == "inbox.list" && size > 1<<20 {
					return fail("inbox too large; use inbox.page")
				}
				out = append(out, d)
			}
		}
		if !start {
			return fail("cursor not found in this inbox")
		}
		if method == "inbox.page" {
			return map[string]any{"messages": out, "nextCursor": next}, nil
		}
		return out, nil
	case "inbox.acknowledge", "inbox.handle", "messages.retry":
		for i := range s.data.Deliveries {
			d := &s.data.Deliveries[i]
			if d.ID != p.MessageID {
				continue
			}
			if !s.deliveryAccess(actor, *d) {
				return fail("delivery unavailable")
			}
			if method == "messages.retry" {
				if actor != "operator" || !p.AcknowledgeDuplicateRisk {
					return fail("operator duplicate-risk acknowledgement required")
				}
				if d.Status != "failed" && d.Status != "interrupted" {
					return fail("only failed or interrupted deliveries can retry")
				}
				if s.taskAbandoned(*d) {
					return fail("abandoned task cannot be reopened by delivery retry")
				}
				if d.Kind == "task" && (s.data.Tasks[d.TaskID].Status == "submitted" || s.data.Tasks[d.TaskID].Status == "accepted") {
					return fail("submitted task requires explicit new revision, not delivery retry")
				}
				d.Status = "pending"
				d.Error = ""
				d.FailureKind = ""
				return *d, nil
			}
			receiver := actor
			if actor == "operator" && p.SessionID != "" {
				receiver = p.SessionID
			}
			if receiver != d.To {
				return fail("receiver required")
			}
			if method == "inbox.handle" {
				if !d.Acknowledged || strings.TrimSpace(p.Evidence) == "" || len(p.Evidence) > 8192 {
					return fail("read acknowledgement and handling evidence (max 8KiB) required")
				}
				if d.Handled && d.HandlingEvidence != p.Evidence {
					return fail("handling evidence immutable")
				}
				d.Handled = true
				d.HandlingEvidence = p.Evidence
				return *d, nil
			}
			d.Acknowledged = true
			if d.Status == "pending" && s.data.Sessions[d.To].Mode != "managed" {
				d.Status = "acknowledged"
			}
			return *d, nil
		}
		return fail("message missing")
	case "context.put", "context.get":
		return s.context(actor, method, p)
	case "tasks.list":
		out := []Task{}
		for _, t := range s.data.Tasks {
			if s.scoped(actor, t.Target) && s.teamAccess(actor, t.Target, t.TeamID) {
				out = append(out, cloneTask(t))
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	case "tasks.get", "tasks.review", "tasks.accept", "tasks.submit":
		t, ok := s.data.Tasks[p.ID]
		if !ok || !s.scoped(actor, t.Target) || !s.teamAccess(actor, t.Target, t.TeamID) {
			return fail("task unavailable")
		}
		if method == "tasks.get" {
			return cloneTask(t), nil
		}
		if !s.taskTeamActive(t) {
			return fail("task team participant left or was revoked; operator coordination required")
		}
		if method == "tasks.submit" {
			if len(p.Output) > 128<<10 {
				return fail("task output exceeds 128KiB; submit an artifact reference")
			}
			if actor != t.Author {
				return fail("task author required")
			}
			if terminalTaskStatus(t.Status) || t.Revision != p.ExpectedRevision || strings.TrimSpace(p.Output) == "" {
				return fail("non-terminal task, matching revision and output required")
			}
			t = s.submit(t, p.Output)
			for i := range s.data.Deliveries {
				d := &s.data.Deliveries[i]
				if d.TaskID == t.ID && d.Kind == "task" && (d.Status == "pending" || d.Status == "acknowledged") {
					d.Status = "completed"
					d.Output = p.Output
				}
			}
			return cloneTask(t), nil
		}
		if method == "tasks.review" {
			if len(p.Evidence) > 8192 {
				return fail("review evidence exceeds 8KiB; use an artifact reference")
			}
			if p.ExpectedRevision != t.Revision {
				return fail("review revision conflict")
			}
			if actor == t.Author || (actor != t.Reviewer && actor != t.RedTeam) {
				return fail("designated independent reviewer required")
			}
			if t.Status != "submitted" {
				return fail("submitted result required")
			}
			if (p.Verdict != "approved" && p.Verdict != "rejected") || strings.TrimSpace(p.Evidence) == "" {
				return fail("verdict and evidence required")
			}
			for _, r := range t.Reviews {
				if r.Actor == actor && r.Revision == t.Revision {
					return fail("review immutable")
				}
			}
			t.Reviews = append(t.Reviews, Review{Actor: actor, Verdict: p.Verdict, Evidence: p.Evidence, Revision: t.Revision})
		} else {
			if actor != "operator" && actor != t.Lead {
				return fail("assigning lead required")
			}
			if t.Status != "submitted" || strings.TrimSpace(t.Output) == "" {
				return fail("submitted output required")
			}
			approved := map[string]bool{}
			for _, r := range t.Reviews {
				if r.Revision == t.Revision && r.Verdict == "approved" {
					approved[r.Actor] = true
				}
			}
			if !approved[t.Reviewer] || (t.RedTeam != "" && !approved[t.RedTeam]) {
				return fail("independent approvals required")
			}
			t.Status = "accepted"
		}
		s.data.Tasks[t.ID] = t
		return cloneTask(t), nil
	default:
		return fail("unknown method")
	}
}
func cloneTask(t Task) Task { t.Reviews = append([]Review{}, t.Reviews...); return t }
func (s *Service) submit(t Task, output string) Task {
	t.Status = "submitted"
	t.Output = output
	t.Revision++
	s.data.Tasks[t.ID] = t
	var parent Delivery
	for _, d := range s.data.Deliveries {
		if d.TaskID == t.ID && d.Kind == "task" {
			parent = d
			break
		}
	}
	s.enqueue(t.Author, t.Lead, output, "result", t.ID, "", 0)
	s.data.Deliveries[len(s.data.Deliveries)-1].TeamID = t.TeamID
	if parent.ID != "" {
		d := &s.data.Deliveries[len(s.data.Deliveries)-1]
		d.ReplyTo = parent.ID
		d.ThreadID = parent.ThreadID
	}
	return t
}
func (s *Service) Claim(agentID string) (*Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("closed")
	}
	v, ok := s.data.Sessions[agentID]
	if !ok {
		return nil, errors.New("unknown session")
	}
	if v.Mode != "managed" || v.Busy {
		return nil, nil
	}
	pending := false
	for _, d := range s.data.Deliveries {
		if d.To == agentID && d.Status == "pending" && s.deliveryRunnable(d) {
			if d.Kind == "task" && v.Policy != "workflow" {
				return nil, errors.New("task author policy requires operator repair before execution")
			}
			pending = true
			break
		}
	}
	if !pending {
		return nil, nil
	}
	res, err := s.mutate(func() (any, error) {
		for i := range s.data.Deliveries {
			d := &s.data.Deliveries[i]
			if d.To == agentID && d.Status == "pending" && s.deliveryRunnable(*d) {
				d.Status = "running"
				d.Attempts++
				v.Busy = true
				s.data.Sessions[v.ID] = v
				if d.Kind == "task" {
					t := s.data.Tasks[d.TaskID]
					t.Status = "working"
					s.data.Tasks[t.ID] = t
				}
				copy := *d
				return &copy, nil
			}
		}
		return nil, nil
	})
	if err != nil || res == nil {
		return nil, err
	}
	return res.(*Delivery), nil
}
func (s *Service) Finish(id, runtimeID, output string, runErr error) error {
	if len(output) > 128<<10 {
		output = ""
		runErr = errors.New("runtime output exceeds 128KiB; use an artifact reference")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("closed")
	}
	_, err := s.mutate(func() (any, error) {
		for i := range s.data.Deliveries {
			d := &s.data.Deliveries[i]
			if d.ID != id {
				continue
			}
			if d.Status != "running" {
				return nil, errors.New("delivery is not running")
			}
			v := s.data.Sessions[d.To]
			v.Busy = false
			if !s.deliveryRunnable(*d) {
				runErr = errors.Join(errors.New("task abandoned or team access changed during execution; result withheld"), runErr)
				output = ""
			}
			if runtimeID != "" && v.RuntimeSessionID != "" && runtimeID != v.RuntimeSessionID {
				// A runner cannot silently rebind a role while reporting a result.
				runErr = errors.Join(errors.New("runtime session identity changed; operator reconciliation required"), runErr)
				runtimeID, output = "", ""
			}
			if runtimeID != "" {
				v.RuntimeSessionID = runtimeID
			}
			s.data.Sessions[v.ID] = v
			d.Output = output
			d.FailureKind = ""
			if runErr == nil && d.Kind == "task" && strings.TrimSpace(output) == "" {
				runErr = errors.New("task output required")
			}
			if runErr != nil {
				d.Status = "failed"
				d.Error = runErr.Error()
				d.FailureKind = "runtime_error"
				if errors.Is(runErr, ErrRuntimeCanceled) {
					d.FailureKind = "canceled"
				}
				if d.Kind == "task" && s.data.Tasks[d.TaskID].Status == "working" {
					t := s.data.Tasks[d.TaskID]
					t.Status = "failed"
					s.data.Tasks[t.ID] = t
				}
				return nil, nil
			}
			d.Status = "completed"
			if d.Kind == "task" {
				t := s.data.Tasks[d.TaskID]
				if t.Status == "working" {
					s.submit(t, output)
				}
			}
			if d.Kind == "message" {
				parent := *d
				s.enqueue(d.To, d.From, output, "result", d.TaskID, "", 0)
				result := &s.data.Deliveries[len(s.data.Deliveries)-1]
				result.TeamID = parent.TeamID
				result.ReplyTo = parent.ID
				result.ThreadID = parent.ThreadID
			}
			return nil, nil
		}
		return nil, errors.New("delivery missing")
	})
	return err
}
