// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"agentcommons/internal/core"
	"agentcommons/internal/harness"
	agentruntime "agentcommons/internal/runtime"
	"agentcommons/internal/transport"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run dispatches and then renders. Attaching the unreachable-service block here
// rather than in each command is what keeps one rendering of it: a command only
// has to return the typed error rpcCall produced, however deep it was raised,
// and this is the single place that decides an operator should read a block
// about it. Commands whose stdout is a machine contract are unaffected -- the
// block goes to errOut, and stdout is never touched.
func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	err := dispatch(ctx, args, in, out, errOut)
	var unreachable *serviceUnreachableError
	if errors.As(err, &unreachable) && humanFacing(args) {
		if renderErr := writeServiceUnreachable(errOut, unreachable); renderErr != nil {
			return renderErr
		}
	}
	return err
}

// humanFacing reports whether this command's reader is a person. The machine
// facing ones -- call, mcp and connect-mcp -- get the single-line Error() their
// caller already prints, because a styled block on their stderr is noise to a
// parser reading their stdout.
func humanFacing(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "call", "mcp", "connect-mcp":
		return false
	}
	return true
}

func dispatch(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) > 0 && args[0] == "harnesses" {
		if len(args) != 1 {
			return errors.New("harnesses takes no arguments")
		}
		return json.NewEncoder(out).Encode(harness.Catalog())
	}
	if len(args) > 0 && args[0] == "codex-prepare" {
		return runCodexPrepareWith(ctx, args[1:], out, errOut, startCodexBootstrap)
	}
	if len(args) > 0 && args[0] == "codex-resume-check" {
		return runCodexResumeCheckWith(ctx, args[1:], out, errOut, startCodexBootstrap)
	}
	if len(args) > 0 && args[0] == "wake-resolve" {
		return runWakeResolution(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "wake-maintain" {
		return runWakeMaintenance(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "launch-context" {
		return runLaunchContext(ctx, args[1:], in, out, errOut)
	}
	if len(args) > 0 && args[0] == "init" {
		return runInit(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "console" {
		return runConsole(ctx, args[1:], in, out, errOut)
	}
	if len(args) > 0 && args[0] == "service" {
		return runService(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "bundle" {
		return runBundle(args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "doctor" {
		return runDoctor(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "service-plan" {
		return runServicePlan(args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "connect-mcp" {
		return runConnectedMCP(ctx, args[1:], in, out, errOut)
	}
	if len(args) > 0 && (args[0] == "enroll" || args[0] == "check-in") {
		return runOnboarding(ctx, args, out, errOut)
	}
	// A bare invocation is the same gesture as asking for help, so it gets the
	// same answer: the help screen, on stdout, exit 0. Typing a binary's name to
	// find out what it does is discovery, not a usage error, and a non-zero exit
	// here makes a shell think the tool is broken.
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return writeHelp(out)
	}
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version") {
		return writeVersion(out)
	}
	if len(args) > 0 && args[0] == "update" {
		return runUpdate(ctx, args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "methods" {
		return json.NewEncoder(out).Encode(transport.Methods())
	}
	if len(args) > 0 && args[0] == "watch" {
		return runWatch(ctx, args[1:], out, errOut)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	state := fs.String("state", filepath.Join(home, ".local", "state", "agent-commons"), "private service state directory")
	socket := fs.String("socket", "", "Unix socket (default STATE/service.sock)")
	tokenFile := fs.String("token-file", "", "private credential file; required for MCP")
	target := fs.String("target", ".", "target repository for discovery or inventory")
	// Deliberately narrower than harness.Catalog()/runtimeVocabulary(): only
	// claude and codex have DiscoverX implemented below; pi and hermes support
	// check-in but not --target discovery yet.
	runtimeName := fs.String("runtime", "claude", "runtime for discovery: claude or codex")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *socket == "" {
		*socket = filepath.Join(*state, "service.sock")
	}
	encode := func(v any) error { return json.NewEncoder(out).Encode(v) }
	switch args[0] {
	case "serve":
		s, err := core.New(*state)
		if err != nil {
			return err
		}
		defer s.Close()
		if err = recoverServiceSocket(*state, *socket); err != nil {
			return err
		}
		token, err := s.Token("operator")
		if err != nil {
			return err
		}
		path := filepath.Join(*state, "operator.token")
		// Do not follow a pre-existing symlink when materializing credentials.
		if info, e := os.Lstat(path); e == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("operator token path must not be a symlink")
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		if err = f.Chmod(0600); err == nil {
			_, err = io.WriteString(f, token+"\n")
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		child, cancel := context.WithCancel(ctx)
		defer cancel()
		results := make(chan error, 2)
		go func() { results <- transport.Serve(child, s, *socket) }()
		go func() {
			results <- agentruntime.Serve(child, s, agentruntime.CLI{Binary: executable, Socket: *socket, StateDir: *state}, 250*time.Millisecond)
		}()
		first := <-results
		cancel()
		second := <-results
		if first != nil && !errors.Is(first, context.Canceled) {
			return first
		}
		if second != nil && !errors.Is(second, context.Canceled) {
			return second
		}
		return nil
	case "call", "mcp":
		if *tokenFile == "" {
			if args[0] == "mcp" {
				return errors.New("mcp requires --token-file for its registered session")
			}
			*tokenFile = filepath.Join(*state, "operator.token")
		}
		token, err := readToken(*tokenFile)
		if err != nil {
			return err
		}
		if args[0] == "mcp" {
			return transport.MCP(ctx, in, out, *socket, token)
		}
		rest := fs.Args()
		if len(rest) < 1 || len(rest) > 2 {
			return errors.New("usage: call [options] METHOD [JSON]; otherwise JSON is read from stdin")
		}
		var raw []byte
		if len(rest) == 2 {
			raw = []byte(rest[1])
		} else {
			raw, err = io.ReadAll(io.LimitReader(in, (2<<20)+1))
			if err != nil {
				return err
			}
		}
		if len(raw) > 2<<20 {
			return errors.New("parameters too large")
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			raw = []byte("{}")
		}
		if !json.Valid(raw) {
			return errors.New("parameters must be valid JSON")
		}
		// json.RawMessage on both ends is what preserves call's stdout contract
		// through the seam. As a parameter it marshals verbatim rather than as
		// the base64 string a plain []byte would become; as the result type its
		// UnmarshalJSON copies the response bytes unchanged, so the round trip
		// is the identity function and this still prints what the service sent.
		result, err := rpcCall[json.RawMessage](ctx,
			rpcClient{socket: *socket, token: token, state: *state}, rest[0], json.RawMessage(raw))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(result))
		return err
	case "discover":
		abs, err := filepath.Abs(*target)
		if err != nil {
			return err
		}
		if *runtimeName == "codex" {
			items, err := agentruntime.DiscoverCodex(ctx, abs)
			if err != nil {
				return err
			}
			return encode(items)
		}
		if *runtimeName != "claude" {
			return errors.New("runtime must be claude or codex")
		}
		items, err := agentruntime.DiscoverClaude(ctx, abs)
		if err != nil {
			return err
		}
		return encode(items)
	case "inventory":
		abs, err := filepath.Abs(*target)
		if err != nil {
			return err
		}
		items, err := agentruntime.Inventory(abs)
		if err != nil {
			return err
		}
		return encode(items)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func readToken(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("credential file must be a regular private file (mode 0600)")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", errors.New("credential too large")
	}
	token := strings.TrimSpace(string(data))
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return "", errors.New("invalid credential file")
	}
	return token, nil
}
