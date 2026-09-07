// SPDX-License-Identifier: MPL-2.0

package codexlaunch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const maxCheckpointBytes = 64 << 10

func readCheckpoint(root *os.Root, name string, value any) error {
	f, err := openExistingBindingFile(root, name, os.O_RDONLY)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCheckpointBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxCheckpointBytes {
		return fmt.Errorf("%s exceeds checkpoint size limit", name)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		return fmt.Errorf("invalid %s checkpoint", name)
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("one checkpoint object required")
	}
	return nil
}
