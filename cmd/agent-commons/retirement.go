// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agentcommons/internal/core"
)

// `retire` and `reinstate` are the operator's registry-maintenance pair. The
// CLI is what created the identities that clutter a registry -- `init` and
// `enroll` -- so the CLI is what has to be able to withdraw one.
//
// Both are orchestration only: parse, one operator RPC, render. Every
// precondition (busy, live attachment lease, participation in an unresolved
// task) is decided in internal/core under its lock, because each one reads
// state only that package owns. Re-deriving any of them here would be both a
// race and a second copy of the rule.

type retirementOptions struct {
	command  string
	state    string
	identity string
	evidence string
	asJSON   bool
}

func runRetirement(ctx context.Context, args []string, out, errOut io.Writer) error {
	options, err := parseRetirement(args, errOut)
	if err != nil {
		return err
	}
	// The same operator credential path enrollment uses; nothing else here
	// needs a connection file, and neither command reads one.
	connection := enrollmentConnection{Config: connectionConfig{
		State: options.state, Socket: filepath.Join(options.state, "service.sock"),
	}}
	client, err := connection.operatorClient()
	if err != nil {
		return err
	}
	session, err := callRetirement(ctx, client, options)
	if err != nil {
		return err
	}
	report := retirementReport(options, session)
	if options.asJSON {
		return json.NewEncoder(out).Encode(report)
	}
	return writeRetirementSummary(out, report)
}

// reinstatement is the shape sessions.reinstate answers with. The credential it
// also returns is deliberately not decoded: the service holds it, and a token
// printed to a terminal is a token in that terminal's scrollback.
type reinstatement struct {
	Session core.Session `json:"session"`
}

func callRetirement(ctx context.Context, client rpcClient, options retirementOptions) (core.Session, error) {
	params := map[string]any{"id": options.identity, "evidence": options.evidence}
	if options.command == "retire" {
		return rpcCall[core.Session](ctx, client, "sessions.retire", params)
	}
	restored, err := rpcCall[reinstatement](ctx, client, "sessions.reinstate", params)
	return restored.Session, err
}

func parseRetirement(args []string, errOut io.Writer) (retirementOptions, error) {
	options := retirementOptions{command: args[0]}
	home, err := os.UserHomeDir()
	if err != nil {
		return options, err
	}
	flags := flag.NewFlagSet(options.command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&options.state, "state", filepath.Join(home, ".local", "state", "agent-commons"), "private service state directory")
	flags.StringVar(&options.identity, "id", "", "identity to "+options.command)
	flags.StringVar(&options.evidence, "evidence", "", "required reason, recorded with the identity")
	flags.BoolVar(&options.asJSON, "json", false, jsonFlagUsage)
	if err := parseFlags(flags, args[1:]); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, errors.New(options.command + " takes no positional arguments")
	}
	if options.identity == "" || strings.TrimSpace(options.evidence) == "" {
		return options, errors.New(options.command + " requires --id and a non-empty --evidence")
	}
	return options, nil
}

// retirementFiles names the private files `enroll` wrote for an identity, and
// says whether it could work them out at all. They are reported, never touched:
// the service has already destroyed the credential inside them, and deleting an
// operator's private file is not this command's decision to make.
//
// prepareEnrollment (enrollment.go) pins the connection PATH to the digest of
// the identity it GUESSED from target + name + role, and deliberately does not
// move it when the server adopts a different, pre-existing session. So the path
// is recoverable only when those two coincide. This recomputes the guess from
// the session's own target, name and role and compares: equal means the derived
// paths are the real ones, and unequal means this command cannot locate the
// connection from the registry and must say so.
//
// Reporting a derived path anyway would be worse than saying nothing. The only
// remedy available after a reinstatement is manual and operates on exactly
// these two paths, so a wrong path does not merely misinform -- it sends a
// repair at a file that does not exist while the real credential sits unnamed
// under the guessed stem.
func retirementFiles(state string, session core.Session) (config, credential string, located bool) {
	guessed := "agent-" + identityDigest(session.Target+"\x00"+session.Name+"\x00"+session.Role)
	if guessed != session.ID {
		return "", "", false
	}
	stem := identityDigest(session.ID)
	return filepath.Join(state, "connection-"+stem+".json"),
		filepath.Join(state, "credential-"+stem+".token"), true
}

const retiredNotice = "The record, its inbox, its board posts and its review verdicts are kept. " +
	"The credential was destroyed. Undelivered messages were marked interrupted, " +
	"and team memberships were revoked."

// reinstatedNotice has to state the consequence, not just the fact. Saying the
// credential file is unchanged is true and useless on its own: what the
// operator needs to know is that the identity CANNOT CONNECT until they fix it
// by hand, and that this command is not the one that hands them the new
// credential.
//
// Neither notice refers to files "above", because whether any are named is
// decided separately -- see the two clauses below.
const reinstatedNotice = "The identity is active again under a new credential the service holds. " +
	"This command does not print it and writes no file, so the enrollment credential file still " +
	"holds the destroyed one and this identity cannot connect until you replace it by hand (mode 0600). " +
	"To get the new credential, reinstate through `agent-commons call --state STATE sessions.reinstate " +
	"'{\"id\":\"...\",\"evidence\":\"...\"}'`, whose response carries it; `enroll` will not rewrite a " +
	"credential file that differs. Team memberships stay revoked; re-invite explicitly."

// locatedFilesNotice is appended when the connection path was derivable, which
// is the only case in which any path was printed.
const locatedFilesNotice = " The connection and credential files named here were not deleted."

// unlocatableNotice is appended instead when the connection was recorded under
// an adopted identity, so its path cannot be derived from the registry.
const unlocatableNotice = " The connection and credential files cannot be located from the registry: " +
	"this identity's ID differs from the target/name/role digest `enroll` pins those paths to, which " +
	"happens when enroll adopted a pre-existing session. They are not named here, and nothing was deleted."

func retirementReport(options retirementOptions, session core.Session) map[string]string {
	notice := retiredNotice
	if options.command == "reinstate" {
		notice = reinstatedNotice
	}
	report := map[string]string{
		"operation": options.command,
		"identity":  session.ID,
		"name":      session.Name,
		"role":      session.Role,
		"target":    session.Target,
		"retiredAt": session.RetiredAt,
		"reason":    session.RetiredReason,
	}
	config, credential, located := retirementFiles(options.state, session)
	if located {
		report["connection"] = config
		report["credential"] = credential
		notice += locatedFilesNotice
	} else {
		notice += unlocatableNotice
	}
	report["notice"] = notice
	return report
}
