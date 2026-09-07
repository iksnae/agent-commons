# Team-scoped work

Teams can now own a task, message or context namespace within a project. Check
`sessions.capabilities` for `teamWorkAvailable` before using this feature with an
older service. Joining a team still grants no repository-write or deployment
authority; workflow policy and independent review requirements remain in force.

Use `teamId` on `context.put`, `context.get`, `messages.send` and `tasks.assign`.
Omit it for existing project-scoped behavior. A team ID is separate from the legacy
session `team` label. The operator creates teams and invites participants; each
participant must join before accessing scoped work.

For a scoped assignment, the lead, author, reviewer and optional red-team reviewer
must all be joined participants. The operator may assign without joining. Task
lookup and listings enforce membership. Submission, review and acceptance also
require all assigned participants to remain joined. Task IDs carry their scope;
do not supply `teamId` again on `tasks.get`, `tasks.submit` or review calls.

## Context and returns

Context IDs have separate project and team namespaces. A project `brief` and a
team `brief` can coexist. On a scoped message or task, `contextId` resolves inside
that team, and `contextVersion` pins an immutable revision. New context revisions
do not rewrite existing deliveries.

Automatic results retain team scope. Explicit replies must send the original
`teamId` along with `replyTo`; omitting it cannot turn a scoped reply into a project
message. Reusing an idempotency key with another scope fails. Managed prompts
include `TeamID` so an agent can preserve it when using messaging tools.

## Leaving and revocation

An absent, invited, left or revoked participant cannot read scoped tasks/context
or scoped inbox entries. Acknowledgement and handling calls enforce the same
boundary. Revoked identities cannot rejoin without a new operator invitation.

Pending work waits while its participants are not joined. An operator retry does
not bypass that check. Finish checks membership again: if a required participant
is absent, the run fails without submitting its output or sending a result.
Retained task evidence remains available to the operator and eligible members.

The supervisor checks active deliveries each polling interval (250 ms by default)
and cancels a run when it observes withdrawn access. The CLI adapter terminates
its owned process group. A canceled run cannot submit a successful result even if
the participant rejoins before the runner returns. Cancellation cannot undo
effects already performed; it is not an instantaneous revocation barrier.

Checks use current membership. Leaving and rejoining entirely between checks
does not invalidate the run. Rejoining can make retained work accessible again.
Revocation cannot erase content already received. Native history can outlive team access, including when one role
joins multiple teams. Separate native/worktree isolation remains a production
gate; RPC access checks are not a confidentiality boundary inside a shared native
session. Peers can also copy information they have already read.

## State compatibility

The first team-scoped write saves a private `pre-team-work-schema-2-<hash>.json`
snapshot, then advances state to schema 3. Failure to preserve the snapshot stops
the write. Starting the new binary alone does not upgrade state. Creating another
team after upgrade does not downgrade it.

Older schema-2 binaries refuse schema 3. Never edit the version number to force
a downgrade. The backup represents the earlier state, not work created since;
restoring it requires an explicit recovery decision and reconciliation of later
effects. Full backup/restore tooling and native upgrade rehearsals remain open.

Tests cover scoped assignment, independent review, immutable context pins,
revocation, withheld results, restart, migration failure and malformed scope
records. Supervisor fixtures verify sticky cancellation after rejoin and termination
of a real child process plus its descendant. They do not prove native Claude/Codex
provider cancellation, tool cleanup outside the owned process group, or autonomous
native team operation.
