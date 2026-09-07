// SPDX-License-Identifier: MPL-2.0

package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOversizedMutationPreservesDurableAndInMemoryState(t *testing.T) {
	s, dir, _ := setup(t)
	path := filepath.Join(dir, "state.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.mutate(func() (any, error) {
		s.data.Contexts["oversized"] = []Context{{Text: strings.Repeat("x", maxStateBytes)}}
		return nil, nil
	})
	if !errors.Is(err, errStateTooLarge) {
		t.Fatal("oversized mutation was not rejected", err)
	}
	if _, ok := s.data.Contexts["oversized"]; ok {
		t.Fatal("rejected mutation retained in memory")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected mutation replaced durable state", err)
	}
}
