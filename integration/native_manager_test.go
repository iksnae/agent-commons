// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type nativeManager struct{ label, file string }

func nativeCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", name, args, err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func (m nativeManager) target() string { return fmt.Sprintf("gui/%d/%s", os.Getuid(), m.label) }

func (m nativeManager) start() error {
	if runtime.GOOS == "darwin" {
		_, err := nativeCommand("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), m.file)
		return err
	}
	if _, err := nativeCommand("systemctl", "--user", "link", "--runtime", m.file); err != nil {
		return err
	}
	_, err := nativeCommand("systemctl", "--user", "start", m.label+".service")
	return err
}

func (m nativeManager) stop() error {
	if runtime.GOOS == "darwin" {
		_, err := nativeCommand("launchctl", "bootout", m.target())
		return err
	}
	_, err := nativeCommand("systemctl", "--user", "stop", m.label+".service")
	return err
}

func (m nativeManager) uninstall() error {
	if runtime.GOOS == "darwin" {
		return nil
	} // bootout removed the transient registration
	if _, err := nativeCommand("systemctl", "--user", "disable", "--runtime", m.label+".service"); err != nil {
		return err
	}
	_, err := nativeCommand("systemctl", "--user", "daemon-reload")
	return err
}

func (m nativeManager) crash() error {
	if runtime.GOOS == "darwin" {
		_, err := nativeCommand("launchctl", "kill", "SIGKILL", m.target())
		return err
	}
	_, err := nativeCommand("systemctl", "--user", "kill", "--signal=KILL", "--kill-whom=main", m.label+".service")
	return err
}

func (m nativeManager) pid() (int, error) {
	var output string
	var err error
	if runtime.GOOS == "darwin" {
		output, err = nativeCommand("launchctl", "print", m.target())
		if err != nil {
			return 0, err
		}
		match := regexp.MustCompile(`(?m)^\s*pid = (\d+)\s*$`).FindStringSubmatch(output)
		if len(match) != 2 {
			return 0, fmt.Errorf("supervisor has no running PID")
		}
		output = match[1]
	} else {
		output, err = nativeCommand("systemctl", "--user", "show", "--property=MainPID", "--value", m.label+".service")
	}
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(output)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("supervisor has no running PID")
	}
	return pid, nil
}
