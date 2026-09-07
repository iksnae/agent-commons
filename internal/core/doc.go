// Package core owns Agent Commons' durable state and authorization boundaries.
//
// Call authenticates an already-resolved actor identity; network transports must
// resolve that identity with Authenticate rather than accepting a claimed sender.
// Token, Claim and Finish are trusted supervisor interfaces, not RPC methods.
//
// Managed delivery progression is pending -> running -> completed/failed. A crash
// converts running to interrupted without replay. Explicit operator retry accepts
// duplicate-effect risk. Acknowledged records receipt independently of execution.
// Completed result deliveries never generate another automatic reply.
//
// tasks.submit accepts {id,output,expectedRevision} from the assigned author and
// advances the result revision atomically. tasks.review requires the exact
// expectedRevision reviewed, a verdict, and evidence from a designated independent
// reviewer. Historical verdicts remain immutable; only the current revision's
// approvals permit acceptance. Acceptance is terminal and does not mean deployed.
//
// Context versions are immutable. Delivery creation pins the requested version,
// or the latest version when omitted, and embeds its content in the delivery.
package core
