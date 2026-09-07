// SPDX-License-Identifier: MPL-2.0

package supervision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledServicePreservesStateAndRefusesChanges(t *testing.T) {
	directory := t.TempDir()
	options := Options{Platform: "linux", Binary: "/opt/agent-commons", State: filepath.Join(t.TempDir(), "state"), SearchPath: "/usr/bin:/bin"}
	file, err := InstallFile(directory, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadInstalled(file); err != nil {
		t.Fatal(err)
	}
	if _, err = InstallFile(directory, options); err == nil {
		t.Fatal("replaced existing installation")
	}
	retained, err := RetainFiles(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadInstalled(filepath.Join(retained, filepath.Base(file))); err != nil {
		t.Fatal("not recoverable", err)
	}
	if _, err = os.Stat(options.State); !os.IsNotExist(err) {
		t.Fatal("installation modified state")
	}
}

func TestChangedServiceCannotBeControlledOrRemoved(t *testing.T) {
	options := Options{Platform: "darwin", Binary: "/opt/agent-commons", State: "/private/state", SearchPath: "/usr/bin"}
	file, err := InstallFile(t.TempDir(), options)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, []byte("keep my changes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadInstalled(file); err == nil {
		t.Fatal("accepted changed file")
	}
	if _, err = RetainFiles(file); err == nil {
		t.Fatal("moved changed file")
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "keep my changes" {
		t.Fatal("modified user data", err)
	}
}

func TestInstallNeverAdoptsExistingMatchingFile(t *testing.T) {
	options := Options{Platform: "linux", Binary: "/opt/agent-commons", State: "/private/state", SearchPath: "/usr/bin"}
	plan, err := Render(options)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	file := filepath.Join(directory, plan.Filename)
	if err = os.WriteFile(file, []byte(plan.Content), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = InstallFile(directory, options); err == nil {
		t.Fatal("adopted existing service")
	}
	if _, err = os.Stat(file + ".json"); !os.IsNotExist(err) {
		t.Fatal("left ownership receipt")
	}
	if _, err = ReadInstalled(file); err == nil {
		t.Fatal("foreign service accepted")
	}
}

func TestIncompleteServiceReceiptCannotAuthorizeControl(t *testing.T) {
	file := filepath.Join(t.TempDir(), "io.agent-commons.0123456789abcdef.service")
	if err := os.WriteFile(file+".json", []byte(`{"version":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInstalled(file); err == nil {
		t.Fatal("incomplete receipt accepted")
	}
}
