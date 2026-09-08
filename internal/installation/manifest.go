// SPDX-License-Identifier: MPL-2.0

package installation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// binaryName is the payload the installer places at the destination root.
const binaryName = "agent-commons"

// mcpManifests declare the plugin's MCP server command. Installation adds
// nothing to PATH, so a bare command would leave every installed plugin unable
// to start its server. Installation rewrites these to the placed binary.
var mcpManifests = []string{"plugins/agent-commons/mcp.json", "plugins/agent-commons/.mcp.json"}

func mcpManifest(path string) bool {
	for _, name := range mcpManifests {
		if path == name {
			return true
		}
	}
	return false
}

func decodeManifest(path string) (map[string]any, error) {
	f, err := regularFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, maxReceipt+1))
	var manifest map[string]any
	if err = d.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("unreadable MCP manifest %s: %w", path, err)
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("one MCP manifest document required: %s", path)
	}
	return manifest, nil
}

// manifestServers returns the mutable server entries keyed by server name.
func manifestServers(path string, manifest map[string]any) (map[string]map[string]any, error) {
	declared, ok := manifest["mcpServers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("MCP manifest declares no servers: %s", path)
	}
	servers := map[string]map[string]any{}
	for name, value := range declared {
		server, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("MCP server %s is not an object: %s", name, path)
		}
		servers[name] = server
	}
	return servers, nil
}

// rewriteManifestCommand returns the manifest with every server command
// replaced by the absolute path of the binary this installation places.
func rewriteManifestCommand(source, binary string) ([]byte, error) {
	manifest, err := decodeManifest(source)
	if err != nil {
		return nil, err
	}
	servers, err := manifestServers(source, manifest)
	if err != nil {
		return nil, err
	}
	for name, server := range servers {
		command, ok := server["command"].(string)
		if !ok || command == "" {
			return nil, fmt.Errorf("MCP server %s declares no command: %s", name, source)
		}
		if filepath.Base(command) != binaryName {
			return nil, fmt.Errorf("MCP server %s names an unexpected command %q: %s", name, command, source)
		}
		server["command"] = binary
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// verifyManifestCommands reports whether an installed manifest still names a
// runnable server binary. Intact bytes alone do not make an install usable.
func verifyManifestCommands(path string) error {
	manifest, err := decodeManifest(path)
	if err != nil {
		return err
	}
	servers, err := manifestServers(path, manifest)
	if err != nil {
		return err
	}
	for name, server := range servers {
		command, ok := server["command"].(string)
		if !ok || !filepath.IsAbs(command) {
			return fmt.Errorf("MCP server %s command is not an absolute path: %s", name, path)
		}
		info, err := os.Stat(command)
		if err != nil {
			return fmt.Errorf("MCP server %s command is missing: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0100 == 0 {
			return fmt.Errorf("MCP server %s command is not an executable file: %s", name, command)
		}
	}
	return nil
}
