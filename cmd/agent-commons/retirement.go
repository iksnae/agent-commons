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

// retirementFiles names the private files `enroll` writes for an identity. They
// are reported, never touched: the service has already destroyed the credential
// inside them, and deleting an operator's private file is not this command's
// decision to make.
//
// The derivation mirrors prepareEnrollment (enrollment.go) for an identity
// enroll minted. It does NOT hold when enroll ADOPTED a pre-existing session
// whose ID differs from the name/role/target guess the path was pinned to; in
// that case these are the paths for the named identity, not the paths that
// connection was recorded at. Nothing here reads them, so a wrong path
// misinforms rather than damages -- but it does misinform, and it is the one
// thing in this command that is not authoritative.
func retirementFiles(state, identity string) (config, credential string) {
	stem := identityDigest(identity)
	return filepath.Join(state, "connection-"+stem+".json"),
		filepath.Join(state, "credential-"+stem+".token")
}

const retiredNotice = "The record, its inbox, its board posts and its review verdicts are kept. " +
	"The credential was destroyed, so the two files above are inert; they were not deleted. " +
	"Undelivered messages were marked interrupted, and team memberships were revoked."

const reinstatedNotice = "The identity is active again under a new credential the service holds. " +
	"The credential file above still holds the destroyed one and was not changed. " +
	"Team memberships stay revoked; re-invite explicitly."

func retirementReport(options retirementOptions, session core.Session) map[string]string {
	config, credential := retirementFiles(options.state, session.ID)
	notice := retiredNotice
	if options.command == "reinstate" {
		notice = reinstatedNotice
	}
	return map[string]string{
		"operation":  options.command,
		"identity":   session.ID,
		"name":       session.Name,
		"role":       session.Role,
		"target":     session.Target,
		"retiredAt":  session.RetiredAt,
		"reason":     session.RetiredReason,
		"connection": config,
		"credential": credential,
		"notice":     notice,
	}
}
