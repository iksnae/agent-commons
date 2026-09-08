// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"io"

	"agentcommons/internal/core"
)

// The human form of the reports the operator-facing commands write. Each
// renderer is handed values a command already holds; none of them re-derives
// anything, makes an RPC, or decides whether it is the form that gets written.
// That choice belongs to the command, on its own --json flag.
//
// The JSON documents these stand beside are a machine contract and are
// unchanged: --json restores each one exactly.

// initSummary is the human half of init's report. It names the same values the
// JSON document carries, as fields rather than map keys, so a renderer cannot
// quietly pick up the wrong one.
type initSummary struct {
	target, manifest, config, state, runtime, identity, identityStatus string
}

// writeInitSummary reports what init settled on. The replacement warning is
// deliberately absent: init already writes it to stderr, which is where it has
// to be for a piped stdout, and repeating it here would print it twice on a
// terminal where both streams land in the same place.
func writeInitSummary(out io.Writer, summary initSummary) error {
	human := newReport(out)
	human.headline("Project set up.")
	human.field("Project", summary.target)
	human.field("Identity", summary.identity+" ("+summary.identityStatus+")")
	human.field("Runtime", summary.runtime)
	human.field("Defaults", summary.manifest)
	human.field("Connection", summary.config)
	human.field("Service state", summary.state)
	human.blank()
	human.note("Next: run 'agent-commons doctor' here to check this connection.")
	return human.write()
}

// writeEnrollmentSummary reports the role connection enroll settled on. It says
// which of adopting and creating happened, because that is the difference an
// operator cannot see anywhere else and the one an accidental second identity
// hides behind.
func writeEnrollmentSummary(out io.Writer, enrolled enrollmentResult) error {
	human := newReport(out)
	human.headline("Role connected.")
	human.field("Identity", enrolled.Identity+" ("+enrolled.IdentityStatus+")")
	human.field("Project", enrolled.Target)
	human.field("Connection", enrolled.Config)
	human.blank()
	human.note("An adopted identity keeps its credential and inbox. No project defaults were written.")
	return human.write()
}

// writeHealthSummary renders a doctor report. Ready and not-ready use the same
// layout: the failing check is named in place, so a reader never has to compare
// two shapes to find out which one they got.
func writeHealthSummary(out io.Writer, report healthReport) error {
	human := newReport(out)
	if report.Ready {
		human.headline("Connection ready.")
	} else {
		human.headline("Connection not ready.")
	}
	if report.Identity != "" {
		human.field("Identity", report.Identity)
	}
	for _, check := range report.Checks {
		human.field(checkOutcome(check), check.Name)
	}
	writeRuntimeSummary(human, report.Runtime)
	human.blank()
	human.note(report.Notice)
	return human.write()
}

// checkOutcome is the label column for one check: its result, not its name, so
// the eye runs down the outcomes rather than the method names.
func checkOutcome(check healthCheck) string {
	if check.OK {
		return "ok"
	}
	if check.Code == "" {
		return "failed"
	}
	return check.Code
}

// writeRuntimeSummary adds the runtime snapshot when the connected service
// advertised one. An older service omits it, and so does this block.
func writeRuntimeSummary(human *report, page *core.RuntimeStatusPage) {
	if page == nil {
		return
	}
	human.blank()
	human.field("Supervisor", page.Supervisor.State)
	for _, session := range page.Sessions {
		human.field("Queue", fmt.Sprintf("%s (%s): %d ready, %d running, %d interrupted, %d failed, %d held by abandoned tasks",
			session.Identity, session.Runtime, session.Ready, session.Running, session.Interrupted, session.Failed, session.WaitingAbandoned))
	}
}

// writeBundleSummary renders one bundle operation. It reads the same map the
// JSON document is encoded from, by the keys the operation actually sets, so
// the two forms cannot report different values.
func writeBundleSummary(out io.Writer, result map[string]string) error {
	human := newReport(out)
	human.headline("Bundle " + bundlePastTense(result["operation"]) + ".")
	human.field("Directory", result["directory"])
	// A slice, not a map: ranging a map here would order two lines by hash seed
	// and change the output between runs.
	for _, optional := range []struct{ label, key string }{{"Binary", "binary"}, {"Moved to", "retained"}} {
		if value := result[optional.key]; value != "" {
			human.field(optional.label, value)
		}
	}
	human.blank()
	human.note(result["notice"])
	return human.write()
}

func bundlePastTense(operation string) string {
	switch operation {
	case "install":
		return "installed"
	case "remove":
		return "removed"
	default:
		return "verified"
	}
}

// serviceSentence is what one unreachable condition says: what is wrong, and
// what to do about it. Both halves are required -- a condition that names
// itself without naming a fix leaves the operator exactly where the raw
// transport error did.
type serviceSentence struct{ headline, fix string }

// serviceSentences maps every condition to its two sentences. It is an array
// indexed by the condition, not a map, so the set is exactly as wide as the
// enum: adding a condition widens this array and the new entry is empty until
// somebody fills it in. Go cannot make that a compile error, so
// TestEveryConditionNamesItselfAndAFix fails on the empty entry instead, and
// serviceConditionUnknown carries a real sentence too so that no path through
// the renderer can produce a blank line.
var serviceSentences = [serviceConditionCount]serviceSentence{
	serviceConditionUnknown: {
		headline: "The local service could not be reached.",
		fix:      "Run 'agent-commons doctor' to check this connection.",
	},
	serviceAbsent: {
		headline: "The local service has never run for this state directory.",
		fix:      "Run 'agent-commons init' in your project to set it up and start the service.",
	},
	serviceStopped: {
		headline: "The local service is not running. It shut down cleanly.",
		fix:      "Start it with 'agent-commons service start', or run 'agent-commons init' in your project.",
	},
	serviceStale: {
		headline: "The local service is not running. It stopped without cleaning up its socket.",
		fix:      "Start it with 'agent-commons service start'; the stale socket is removed on startup.",
	},
}

// serviceUnreachableLine is the machine-facing form: one line, on stderr,
// naming the condition and the fix. It is what Error() returns, so a command
// that simply returns the error still says something useful.
func serviceUnreachableLine(unreachable *serviceUnreachableError) string {
	sentence := serviceSentences[unreachable.condition]
	line := sentence.headline + " " + sentence.fix
	if unreachable.socket != "" {
		line += " (socket " + unreachable.socket + ")"
	}
	return line
}

// writeServiceUnreachable is the human-facing form: the same two sentences as a
// styled block, with the paths a reader may need to check. It goes to stderr,
// never stdout -- stdout is the machine contract on every command that has one,
// and it was never where a diagnosis belonged.
func writeServiceUnreachable(errOut io.Writer, unreachable *serviceUnreachableError) error {
	sentence := serviceSentences[unreachable.condition]
	human := newReport(errOut)
	human.headline(sentence.headline)
	if unreachable.socket != "" {
		human.field("Socket", unreachable.socket)
	}
	if unreachable.state != "" {
		human.field("Service state", unreachable.state)
	}
	human.blank()
	human.note(sentence.fix)
	return human.write()
}

// writeCheckInHeader names who checked in and what they attached to.
//
// It goes to the ERROR stream, and must stay there. check-in's stdout carries
// exactly one JSON document and nothing else, unconditionally: the Pi extension
// parses it whole (plugins/agent-commons/pi/commons.mjs) and --hold streams
// arrivals onto the same writer afterwards. A header on stdout would break the
// first and interleave with the second. That is also why check-in has no --json
// flag and no terminal test on stdout -- there is nothing to switch between.
func writeCheckInHeader(errOut io.Writer, config connectionConfig, attachment core.Attachment) error {
	human := newReport(errOut)
	human.headline("Checked in " + config.Name + " (" + config.Role + ").")
	human.field("Project", config.Target)
	human.field("Attached", attachment.Runtime+" session "+attachment.NativeID)
	human.blank()
	human.note("Nothing was acknowledged, and no team was joined.")
	return human.write()
}
