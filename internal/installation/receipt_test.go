// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiptSizeIsBoundedBeforeInstallation(t *testing.T) {
	r := receipt{Version: 1, Files: []fileRecord{{Path: strings.Repeat("x", maxReceipt)}}}
	if _, err := encodeReceipt(r); err == nil {
		t.Fatal("oversized receipt accepted")
	}
}

func TestVerifyRejectsChangedPermissions(t *testing.T) {
	target := filepath.Join(t.TempDir(), "installed")
	if err := Install(bundleFixture(t), target); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(target, "agent-commons"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(target); err == nil {
		t.Fatal("permission change accepted")
	}
}

func TestVerifyRejectsMalformedReceipts(t *testing.T) {
	for _, data := range []string{`{"version":99,"files":[]}`, `{} {}`, `{"version":1,"unknown":true}`, strings.Repeat("x", maxReceipt+1)} {
		target := filepath.Join(t.TempDir(), "installed")
		if err := Install(bundleFixture(t), target); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, receiptName), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := Verify(target); err == nil {
			t.Fatal("bad receipt accepted")
		}
	}
}
