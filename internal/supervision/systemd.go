// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"fmt"
	"strings"
)

// Unit specifiers and command environment expansion are separate from quoting.
func unitQuote(s string) string {
	s = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "%", "%%", "\t", "\\t").Replace(s)
	return "\"" + s + "\""
}

func systemd(o Options) string {
	return fmt.Sprintf(`[Unit]
Description=Agent Commons local coordination service
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=exec
ExecStart=:%s serve --state %s
Environment=%s
Restart=on-failure
RestartSec=10
TimeoutStopSec=30
KillMode=control-group
UMask=0077

[Install]
WantedBy=default.target
`, unitQuote(o.Binary), unitQuote(o.State), unitQuote("PATH="+o.SearchPath))
}
