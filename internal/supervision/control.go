// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"context"
	"fmt"
	"path/filepath"
)

// Commander is the process boundary; implementations must bound runtime/output.
type Commander interface {
	Run(context.Context, string, ...string) (string, error)
}

type Control struct {
	Platform string
	UID      int
	File     string
	Label    string
	Commands Commander
}

// Apply controls only the explicitly named user job. It never removes files.
func (c Control) Apply(ctx context.Context, operation string) (string, error) {
	commands, err := c.commands(operation)
	if err != nil {
		return "", err
	}
	if c.Commands == nil {
		return "", fmt.Errorf("supervisor command runner required")
	}
	var output string
	for _, command := range commands {
		output, err = c.Commands.Run(ctx, command[0], command[1:]...)
		if err != nil {
			return "", fmt.Errorf("%s failed; supervisor state may be partially changed: %w", operation, err)
		}
	}
	return output, nil
}

func (c Control) commands(operation string) ([][]string, error) {
	if !safePath(c.File) || c.UID < 0 || filepath.Base(c.File) != c.Label+map[string]string{"darwin": ".plist", "linux": ".service"}[c.Platform] {
		return nil, fmt.Errorf("invalid supervisor job identity")
	}
	if len(c.Label) != len("io.agent-commons.")+16 || c.Label[:len("io.agent-commons.")] != "io.agent-commons." {
		return nil, fmt.Errorf("invalid Agent Commons service label")
	}
	for _, ch := range c.Label[len("io.agent-commons."):] {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return nil, fmt.Errorf("invalid service label digest")
		}
	}
	if c.Platform == "darwin" {
		domain := fmt.Sprintf("gui/%d", c.UID)
		switch operation {
		case "start":
			return [][]string{{"/bin/launchctl", "bootstrap", domain, c.File}}, nil
		case "stop":
			return [][]string{{"/bin/launchctl", "bootout", domain + "/" + c.Label}}, nil
		case "status":
			return [][]string{{"/bin/launchctl", "print", domain + "/" + c.Label}}, nil
		case "enable":
			return [][]string{{"/bin/launchctl", "enable", domain + "/" + c.Label}}, nil
		case "disable":
			return [][]string{{"/bin/launchctl", "disable", domain + "/" + c.Label}}, nil
		}
	}
	if c.Platform == "linux" {
		unit := c.Label + ".service"
		switch operation {
		case "reload":
			return [][]string{{"/usr/bin/systemctl", "--user", "daemon-reload"}}, nil
		case "enable":
			return [][]string{{"/usr/bin/systemctl", "--user", "daemon-reload"}, {"/usr/bin/systemctl", "--user", "enable", c.File}}, nil
		case "disable":
			return [][]string{{"/usr/bin/systemctl", "--user", "disable", unit}, {"/usr/bin/systemctl", "--user", "daemon-reload"}}, nil
		case "start", "stop":
			return [][]string{{"/usr/bin/systemctl", "--user", operation, unit}}, nil
		case "status":
			return [][]string{{"/usr/bin/systemctl", "--user", "show", "--property=LoadState,ActiveState,SubState,MainPID", unit}}, nil
		}
	}
	return nil, fmt.Errorf("unsupported platform or service operation")
}
