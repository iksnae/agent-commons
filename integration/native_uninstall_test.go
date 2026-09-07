// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (m nativeManager) verifyUninstalled() error {
	if runtime.GOOS == "darwin" {
		_, err := nativeCommand("launchctl", "print", m.target())
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 113 && strings.Contains(err.Error(), "Could not find service") {
			return nil
		}
		return fmt.Errorf("could not confirm launchd registration absent: %v", err)
	}
	state, err := nativeCommand("systemctl", "--user", "show", "--property=LoadState", "--value", m.label+".service")
	if err != nil || state != "not-found" {
		return fmt.Errorf("could not confirm systemd unit absent: %q %v", state, err)
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	_, err = os.Lstat(filepath.Join(runtimeDir, "systemd", "user", m.label+".service"))
	if !os.IsNotExist(err) {
		return fmt.Errorf("could not confirm runtime link absent: %v", err)
	}
	return nil
}
