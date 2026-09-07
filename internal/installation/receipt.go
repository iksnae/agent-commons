// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"encoding/json"
	"fmt"
)

const maxReceipt = 1 << 20

type receipt struct {
	Version int          `json:"version"`
	Files   []fileRecord `json:"files"`
}

func encodeReceipt(r receipt) ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(data)+1 > maxReceipt {
		return nil, fmt.Errorf("installation receipt exceeds 1 MiB")
	}
	return data, nil
}
