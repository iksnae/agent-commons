// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"agentcommons/internal/core"
)

type enrollmentConnection struct {
	Path   string
	Config connectionConfig
}

// Labels for enrollmentResult.IdentityStatus. These are display strings for a
// human reading a report, nothing more — see the warning on the field.
const (
	identityStatusAdopted = "adopted"
	identityStatusCreated = "created"
	identityStatusUnknown = "unknown"
)

// enrollmentResult reports what enrollAgent settled on, for callers that must
// tell the operator what happened: the real identity the server authenticated
// this credential as, and where the connection was recorded.
type enrollmentResult struct {
	Identity string
	Config   string
	// IdentityStatus is "adopted", "created", or "unknown" when the registry
	// could not be read. It is a display label and NOTHING may branch on it.
	// It is derived by comparing the settled identity against a registry
	// snapshot taken before the enroll RPC, so a concurrent enrollment of the
	// same identity can race it. That race is acceptable for exactly as long
	// as this value only ever reaches a human. Gating any behaviour on it —
	// skipping a write, choosing a code path, failing a check — turns a
	// cosmetic staleness into a defect. If you need an authoritative answer,
	// take the server-side one described at registeredIdentities instead.
	IdentityStatus string
}

func enrollAgent(ctx context.Context, options onboardingOptions, out io.Writer) (enrollmentResult, error) {
	connection, err := prepareEnrollment(options)
	if err != nil {
		return enrollmentResult{}, err
	}
	// Snapshot the registry BEFORE the RPC. sessions.enroll answers with the
	// same shape whether it adopted a pre-existing identity or minted a new
	// one, so "was this ID already registered a moment ago" is the only honest
	// test available here — and it asks the server rather than re-deriving its
	// target/role/name resolution rules on this side, where they would drift.
	//
	// Best effort, deliberately: the snapshot only labels the outcome for a
	// human, so failing to read it must not fail an enrollment that would
	// otherwise succeed. `enroll` discards the label entirely (onboarding.go)
	// and must not acquire a new failure mode on its account. An unreadable
	// registry leaves the label "unknown".
	registered, snapshotErr := connection.registeredIdentities(ctx)
	adopted, credential, err := connection.enrollIdentity(ctx, options.Team)
	if err != nil {
		return enrollmentResult{}, err
	}
	// The server, not the client, decides which session this credential
	// authenticates as: sessions.enroll may adopt a pre-existing session
	// whose ID differs from the pre-RPC guess computed in prepareEnrollment.
	// Correct the in-memory identity to that real, adopted ID before it is
	// compared against (or written into) the connection file. connection.Path
	// and connection.Config.TokenFile are concrete strings fixed above and
	// are deliberately unaffected by this — see the doc comment on
	// connectionConfig.Identity for why the two must stay independent.
	if adopted != "" && adopted != connection.Config.Identity {
		connection.Config.Identity = adopted
	}
	exists, err := connection.checkExisting()
	if err != nil {
		return enrollmentResult{}, err
	}
	if err = ensureCredential(connection.Config.TokenFile, credential); err != nil {
		return enrollmentResult{}, err
	}
	if !exists {
		if err = connection.writeConfig(); err != nil {
			return enrollmentResult{}, err
		}
	}
	status := identityStatusUnknown
	if snapshotErr == nil {
		status = identityStatusCreated
		if registered[connection.Config.Identity] {
			status = identityStatusAdopted
		}
	}
	result := enrollmentResult{
		Identity: connection.Config.Identity, Config: connection.Path,
		IdentityStatus: status,
	}
	return result, json.NewEncoder(out).Encode(map[string]string{
		"identity": connection.Config.Identity, "config": connection.Path, "target": connection.Config.Target,
	})
}

// prepareEnrollment computes the connection file's PATH and a starting-guess
// Config.Identity before any RPC happens, since ambient resolution
// (deriveEnrolledConnectionPath in connection_resolve.go) must be able to
// reproduce the path from a manifest alone, with no server round trip. The
// guessed identity is provisional: enrollAgent corrects Config.Identity
// after the RPC to whatever session sessions.enroll actually adopted, which
// may be a different, pre-existing ID. The path never follows that
// correction — do not make it do so.
func prepareEnrollment(options onboardingOptions) (enrollmentConnection, error) {
	if options.Name == "" || options.Role == "" || !filepath.IsAbs(options.Target) {
		return enrollmentConnection{}, errors.New("enroll requires --name --role and absolute --target")
	}
	target, err := filepath.EvalSymlinks(options.Target)
	if err != nil {
		return enrollmentConnection{}, err
	}
	if options.Identity == "" {
		options.Identity = "agent-" + identityDigest(target+"\x00"+options.Name+"\x00"+options.Role)
	}
	stem := identityDigest(options.Identity)
	if options.Config == "" {
		options.Config = filepath.Join(options.State, "connection-"+stem+".json")
	}
	connection := enrollmentConnection{Path: options.Config, Config: connectionConfig{
		Version: 1, Identity: options.Identity, Target: target, Name: options.Name, Role: options.Role,
		Socket: filepath.Join(options.State, "service.sock"), State: options.State,
		TokenFile: filepath.Join(filepath.Dir(options.Config), "credential-"+stem+".token"),
	}}
	return connection, validateConfigLocation(connection.Path)
}

func identityDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:16])
}

func validateConfigLocation(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("absolute --config required")
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("config parent must be a real private directory")
	}
	return nil
}

func (connection enrollmentConnection) checkExisting() (bool, error) {
	if _, err := os.Lstat(connection.Path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	var previous connectionConfig
	if err := privateRead(connection.Path, &previous); err != nil {
		return false, err
	}
	if previous != connection.Config {
		return false, errors.New("existing config differs; refusing overwrite")
	}
	return true, nil
}

// enrollIdentity performs the sessions.enroll RPC and returns both the
// adopted session's real ID and its credential. The server may adopt a
// pre-existing session (e.g. a legacy blank-Name registration) whose ID
// differs from config.Identity, the pre-RPC guess; callers must treat the
// returned ID as authoritative and correct their Config.Identity to it.
func (connection enrollmentConnection) enrollIdentity(ctx context.Context, team string) (adoptedID, token string, err error) {
	config := connection.Config
	client, err := connection.operatorClient()
	if err != nil {
		return "", "", err
	}
	identity := core.Session{ID: config.Identity, Name: config.Name, Role: config.Role, Team: team,
		Target: config.Target, Runtime: "manual", Mode: "manual", Policy: "coordination"}
	result, err := rpcCall[struct {
		Session core.Session `json:"session"`
		Token   string       `json:"token"`
	}](ctx, client, "sessions.enroll", identity)
	if err != nil {
		return "", "", err
	}
	if result.Token == "" || result.Session.ID == "" {
		return "", "", errors.New("missing enrollment credential")
	}
	return result.Session.ID, result.Token, nil
}

func (connection enrollmentConnection) operatorClient() (rpcClient, error) {
	operator, err := readToken(filepath.Join(connection.Config.State, "operator.token"))
	if err != nil {
		return rpcClient{}, err
	}
	return rpcClient{socket: connection.Config.Socket, token: operator}, nil
}

// registeredIdentities returns the session IDs the service already knows, so
// an enrollment can tell an adopted identity from one it just minted.
//
// This is the second-best answer, taken deliberately. The authoritative one is
// server-side: internal/core's sessions.enroll handler already distinguishes
// adopting an existing session from minting a new one, atomically and under its
// own lock, and an `adopted` boolean in that response would be race-free and
// would delete this function and its extra round trip outright. It was not
// taken here because internal/** was outside this change's blast radius, not
// because it is worse. Whoever next has cause to touch that handler should
// prefer it and retire this snapshot.
func (connection enrollmentConnection) registeredIdentities(ctx context.Context) (map[string]bool, error) {
	client, err := connection.operatorClient()
	if err != nil {
		return nil, err
	}
	peers, err := rpcCall[[]core.Session](ctx, client, "sessions.list", struct{}{})
	if err != nil {
		return nil, err
	}
	registered := make(map[string]bool, len(peers))
	for _, peer := range peers {
		registered[peer.ID] = true
	}
	return registered, nil
}

func ensureCredential(path, expected string) error {
	existing, err := readToken(path)
	if errors.Is(err, os.ErrNotExist) {
		return createPrivate(path, []byte(expected+"\n"))
	}
	if err != nil {
		return err
	}
	if existing != expected {
		return errors.New("credential mismatch; refusing overwrite")
	}
	return nil
}

func (connection enrollmentConnection) writeConfig() error {
	data, err := json.MarshalIndent(connection.Config, "", "  ")
	if err != nil {
		return err
	}
	return createPrivate(connection.Path, data)
}
