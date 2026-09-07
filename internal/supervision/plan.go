// SPDX-License-Identifier: MPL-2.0

// Package supervision renders user-service plans without installing them.
package supervision

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Options struct {
	Platform   string
	Binary     string
	State      string
	SearchPath string
}

type Plan struct {
	Label    string `json:"label"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Notice   string `json:"notice"`
}

func Render(o Options) (Plan, error) {
	for name, value := range map[string]string{"binary": o.Binary, "state": o.State} {
		if !safePath(value) {
			return Plan{}, fmt.Errorf("%s must be an absolute path without control characters", name)
		}
	}
	if filepath.Clean(o.State) == "/" {
		return Plan{}, fmt.Errorf("state must be a dedicated private directory")
	}
	for _, path := range strings.Split(o.SearchPath, ":") {
		if !safePath(path) {
			return Plan{}, fmt.Errorf("PATH entries must be explicit absolute directories")
		}
	}
	o.Binary, o.State = filepath.Clean(o.Binary), filepath.Clean(o.State)
	id := sha256.Sum256([]byte(o.State))
	p := Plan{Label: fmt.Sprintf("io.agent-commons.%x", id[:8]),
		Notice: "Plan only: no files installed or processes started. Use a private state directory; stop the existing service before enabling this job. This supervises the service, not role check-ins or wake watchers."}
	switch o.Platform {
	case "darwin":
		p.Filename = p.Label + ".plist"
		p.Content = launchd(p.Label, o)
	case "linux":
		p.Filename = p.Label + ".service"
		p.Content = systemd(o)
	default:
		return Plan{}, fmt.Errorf("platform must be darwin or linux")
	}
	return p, nil
}

func safePath(value string) bool {
	return filepath.IsAbs(value) && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) == -1
}
