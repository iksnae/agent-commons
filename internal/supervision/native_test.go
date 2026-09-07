// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeServiceParser(t *testing.T) {
	o := validOptions()
	o.Platform = runtime.GOOS
	var err error
	o.Binary, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := Render(o)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), p.Filename)
	if err = os.WriteFile(file, []byte(p.Content), 0600); err != nil {
		t.Fatal(err)
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("plutil", "-lint", file)
	case "linux":
		if _, err = exec.LookPath("systemd-analyze"); err != nil {
			t.Skip("systemd-analyze unavailable")
		}
		command = exec.Command("systemd-analyze", "--user", "verify", file)
	default:
		t.Skip("unsupported host")
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native parser: %v: %s", err, output)
	}
}
