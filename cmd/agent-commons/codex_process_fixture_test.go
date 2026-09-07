// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// This subprocess fixture has no provider or installed-agent dependency. Its
// descendant keeps an independent connection open until killed or test cleanup.
func TestCodexProcessFixture(t *testing.T) {
	args := os.Args
	if len(args) < 4 || args[len(args)-3] != "--" {
		return
	}
	mode, address := args[len(args)-2], args[len(args)-1]
	if mode == "child" {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			os.Exit(2)
		}
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
		_, _ = conn.Write([]byte("R"))
		var stop [1]byte
		_, _ = conn.Read(stop[:])
		_ = conn.Close()
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	child := exec.Command(executable, "-test.run", "^TestCodexProcessFixture$", "--", "child", address)
	if err = child.Start(); err != nil {
		os.Exit(2)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			os.Exit(2)
		}
		if mode == "ready" && request.Method == "initialize" {
			if json.NewEncoder(os.Stdout).Encode(map[string]any{"id": request.ID, "result": map[string]any{}}) != nil {
				os.Exit(2)
			}
		}
	}
	os.Exit(0)
}
