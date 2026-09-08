// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type projectDefaults struct {
	Version     int    `json:"version"`
	ProjectRoot string `json:"projectRoot"`
	Team        string `json:"team"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Runtime     string `json:"runtime"`
	State       string `json:"state"`
}

func runInit(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	target, _ := os.Getwd()
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	state := filepath.Join(home, ".local", "state", "agent-commons")
	name, role, team, runtimeName := "lead", "workspace-lead", "", "codex"
	fs.StringVar(&target, "target", target, "project/workspace directory (default current directory)")
	fs.StringVar(&state, "state", state, "private persistent service state directory")
	fs.StringVar(&name, "name", name, "stable agent name")
	fs.StringVar(&role, "role", role, "project role")
	fs.StringVar(&team, "team", team, "project team label")
	fs.StringVar(&runtimeName, "runtime", runtimeName, "runtime: "+initRuntimeVocabulary())
	asJSON := fs.Bool("json", false, jsonFlagUsage)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || name == "" || role == "" {
		return errors.New("init requires --name and --role and no positional arguments")
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return err
	}
	if !initRuntimeAllowed(runtimeName) {
		return fmt.Errorf("runtime must be %s", initRuntimeVocabulary())
	}
	defaults := projectDefaults{Version: 1, ProjectRoot: target, Team: team, Name: name, Role: role, Runtime: runtimeName, State: state}
	projectDir := filepath.Join(target, ".agent-commons")
	manifest := filepath.Join(projectDir, "project.json")
	// Read the recorded defaults before anything is created, enrolled or
	// started, so the warning can name what this run is about to replace.
	warning, err := reviewProjectDefaults(manifest, defaults)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(state, 0700); err != nil {
		return err
	}
	if err := os.Chmod(state, 0700); err != nil {
		return err
	}
	socket := filepath.Join(state, "service.sock")
	var startup *serviceStartup
	if !serviceReachable(socket) {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		startup, err = startDetached(exec.Command(binary, "serve", "--state", state), state)
		if err != nil {
			return fmt.Errorf("start service: %w", err)
		}
		// Removed on every path out of here, reached or not. A service that came
		// up keeps writing into the unlinked file; one that failed has already
		// been read by then.
		defer startup.discard()
	}
	deadline := time.Now().Add(3 * time.Second)
	for !serviceReachable(socket) && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	if !serviceReachable(socket) {
		return unreachableAfterStart(startup)
	}
	// enrollAgent owns the connection-file guard: it can only be applied once
	// the server has said which identity it really adopted. Do not re-check it
	// here against the pre-RPC guess — that is what stopped init re-pointing a
	// project at any adopted identity.
	//
	// This is NOT transactional with the manifest write below. Once enrollAgent
	// returns, a session has been minted or adopted and a connection file
	// exists; every step after this point can still fail and leave that
	// identity behind with no manifest and no report naming it. A target that
	// is readable and traversable but not writable reaches exactly that state:
	// MkdirAll(projectDir) fails and one session has been minted. That is the
	// stray-identity class the operator's original incident came from. It
	// predates this change, which neither widens nor closes the window.
	// Closing it means either enrolling after the manifest is written or
	// unwinding the enrollment on failure; both are decisions beyond this fix,
	// and TestInitFailureLeavesRecordedDefaultsIntact deliberately does not
	// claim otherwise.
	options := onboardingOptions{Command: "enroll", State: state, Name: name, Role: role, Team: team, Target: target}
	enrolled, err := enrollAgent(ctx, options)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(manifest, append(data, '\n'), 0600); err != nil {
		return err
	}
	// Report the replacement on stderr as well as in the JSON below: stdout is
	// routinely piped into a parser, and this notice is the only thing standing
	// between the operator and a silently re-pointed project.
	if warning != "" {
		if _, err := fmt.Fprintln(errOut, "warning: "+warning); err != nil {
			return err
		}
	}
	// One set of values, two forms. The JSON document is a machine contract and
	// is unchanged; --json is the only way to get it now that a person reading
	// the terminal is the default reader.
	summary := initSummary{target: target, manifest: manifest, config: enrolled.Config,
		state: state, runtime: runtimeName, identity: enrolled.Identity, identityStatus: enrolled.IdentityStatus}
	if !*asJSON {
		return writeInitSummary(out, summary)
	}
	report := map[string]string{"operation": "init", "target": summary.target, "manifest": summary.manifest,
		"config": summary.config, "state": summary.state, "runtime": summary.runtime,
		"identity": summary.identity, "identityStatus": summary.identityStatus}
	if warning != "" {
		report["warning"] = warning
	}
	return json.NewEncoder(out).Encode(report)
}

// reviewProjectDefaults compares a project's recorded defaults with the ones
// init is about to write and names every difference. A difference does not stop
// init: it is the documented way to re-point a project, and the operator chose
// to be warned rather than blocked. The manifest is therefore replaced and the
// previous defaults are gone; the returned warning is the only notice that this
// happened, and it necessarily arrives after the write. An empty warning means
// the recorded defaults were absent or identical.
func reviewProjectDefaults(manifest string, defaults projectDefaults) (string, error) {
	body, err := os.ReadFile(manifest)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var previous projectDefaults
	if err := json.Unmarshal(body, &previous); err != nil {
		return "", fmt.Errorf("unreadable project manifest %s: %w", manifest, err)
	}
	difference := describeProjectDefaultsDifference(previous, defaults)
	if difference == "" {
		return "", nil
	}
	return "replaced existing project defaults: " + difference, nil
}

// projectDefaults must stay comparable field by field. The walk below tests
// equality through reflect.Value.Interface(), which is a RUNTIME comparison: it
// replaced a compile-time `previous == defaults`, so a field whose type is not
// comparable — a slice, a map, a func — would no longer fail the build, it
// would panic inside init. This anchor restores the build failure at the point
// such a field is added. Delete it only by making the walk handle the field
// type it fails on, never by making it compile.
var _ = projectDefaults{} == projectDefaults{}

// describeProjectDefaultsDifference reports every field in which two manifests
// differ, as `field "old" -> "new"`. Detecting the difference and describing it
// are the same walk over the same fields, so the warning can never fire while
// naming nothing, and a field added to projectDefaults is covered by both at
// once. Two lists kept in step by hand is exactly the drift that put this
// command in the state it was in; an empty result means identical, field for
// field, which is what the caller treats as "nothing to warn about".
//
// That automatic coverage is the reason for the anchor above: it invites new
// fields, and the trade taken for it was compile-time safety for runtime
// reflection. Every field is comparable today, so the panic is latent, but the
// invitation and the hazard belong in the same place.
func describeProjectDefaultsDifference(previous, next projectDefaults) string {
	fields := reflect.TypeOf(projectDefaults{})
	before, after := reflect.ValueOf(previous), reflect.ValueOf(next)
	differences := make([]string, 0, fields.NumField())
	for index := 0; index < fields.NumField(); index++ {
		old, current := before.Field(index), after.Field(index)
		if old.Interface() == current.Interface() {
			continue
		}
		name := fields.Field(index).Tag.Get("json")
		if name == "" {
			name = fields.Field(index).Name
		}
		differences = append(differences, fmt.Sprintf("%s %s -> %s", name, describeFieldValue(old), describeFieldValue(current)))
	}
	return strings.Join(differences, "; ")
}

func describeFieldValue(value reflect.Value) string {
	if value.Kind() == reflect.String {
		return strconv.Quote(value.String())
	}
	return fmt.Sprint(value.Interface())
}

// unreachableAfterStart explains a service that never came up. When the child
// said why before exiting, that reason IS the explanation and the old advice to
// go looking in the state directory is dropped: the directory did not contain
// it, which is what made the original message a dead end. When there is nothing
// captured -- the service was already running under someone else, or died
// silently -- the message falls back to naming the state directory, which is
// still the only place left to look.
func unreachableAfterStart(startup *serviceStartup) error {
	if startup == nil {
		return errors.New("service did not become reachable; inspect the private state directory")
	}
	if diagnosis := startup.diagnose(); diagnosis != "" {
		return fmt.Errorf("service did not become reachable: %s", diagnosis)
	}
	return errors.New("service did not become reachable and reported nothing; inspect the private state directory")
}

func serviceReachable(socket string) bool {
	conn, err := net.DialTimeout("unix", socket, 50*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
