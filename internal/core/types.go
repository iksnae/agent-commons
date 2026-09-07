// SPDX-License-Identifier: MPL-2.0

package core

// Board posts are attributed peer knowledge, not instructions or verified facts.
type BoardPost struct {
	ID              string `json:"id"`
	Target          string `json:"target"`
	Author          string `json:"author"`
	Topic           string `json:"topic"`
	Title           string `json:"title"`
	Text            string `json:"text"`
	Evidence        string `json:"evidence"`
	ReplyTo         string `json:"replyTo,omitempty"`
	CreatedAt       string `json:"createdAt"`
	GrantsAuthority bool   `json:"grantsAuthority"`
}

type Session struct {
	Name             string     `json:"name"`
	Attachment       Attachment `json:"attachment"`
	Policy           string     `json:"policy"`
	ID               string     `json:"id"`
	Target           string     `json:"target"`
	Team             string     `json:"team"`
	Role             string     `json:"role"`
	Runtime          string     `json:"runtime"`
	Mode             string     `json:"mode"`
	RuntimeSessionID string     `json:"runtimeSessionId"`
	Instructions     string     `json:"instructions"`
	Busy             bool       `json:"busy"`
}

type Attachment struct {
	Runtime   string `json:"runtime"`
	NativeID  string `json:"nativeId"`
	LeaseID   string `json:"leaseId"`
	ExpiresAt int64  `json:"expiresAt"`
}
type Delivery struct {
	TeamID           string `json:"teamId,omitempty"`
	Provenance       string `json:"provenance"`
	GrantsAuthority  bool   `json:"grantsAuthority"`
	ReplyTo          string `json:"replyTo,omitempty"`
	ThreadID         string `json:"threadId"`
	Handled          bool   `json:"handled"`
	HandlingEvidence string `json:"handlingEvidence,omitempty"`
	Acknowledged     bool   `json:"acknowledged"`
	ID               string `json:"id"`
	From             string `json:"from"`
	To               string `json:"to"`
	Text             string `json:"text"`
	Kind             string `json:"kind"`
	TaskID           string `json:"taskId,omitempty"`
	ContextID        string `json:"contextId,omitempty"`
	ContextVersion   int    `json:"contextVersion,omitempty"`
	Status           string `json:"status"`
	Attempts         int    `json:"attempts"`
	Output           string `json:"output,omitempty"`
	Error            string `json:"error,omitempty"`
	FailureKind      string `json:"failureKind,omitempty"`
	CreatedAt        string `json:"createdAt"`
}
type Review struct {
	Actor    string `json:"actor"`
	Verdict  string `json:"verdict"`
	Evidence string `json:"evidence"`
	Revision int    `json:"revision"`
}
type Task struct {
	TeamID   string   `json:"teamId,omitempty"`
	ID       string   `json:"id"`
	Target   string   `json:"target"`
	Lead     string   `json:"lead"`
	Author   string   `json:"author"`
	Title    string   `json:"title"`
	Criteria string   `json:"criteria"`
	Reviewer string   `json:"reviewer"`
	RedTeam  string   `json:"redTeam,omitempty"`
	Status   string   `json:"status"`
	Output   string   `json:"output,omitempty"`
	Revision int      `json:"revision"`
	Reviews  []Review `json:"reviews"`
}
type Context struct {
	TeamID  string `json:"teamId,omitempty"`
	ID      string `json:"id"`
	Target  string `json:"target"`
	Text    string `json:"text"`
	Version int    `json:"version"`
}
