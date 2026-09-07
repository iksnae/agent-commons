package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Definition retains its original location so relative skill references resolve.
type Definition struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Runtime string `json:"runtime"`
	Path    string `json:"path"`
	BaseDir string `json:"baseDir"`
	Digest  string `json:"digest"`
	Content string `json:"content"`
}

func Inventory(target string) ([]Definition, error) {
	target, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target is not a directory")
	}
	var out []Definition
	var totalBytes int64
	for _, runtime := range []string{"claude", "codex", "agencyx"} {
		for _, kind := range []string{"agents", "skills", "commands"} {
			root := filepath.Join(target, "."+runtime, kind)
			// Resolve project-owned directory links without copying files or losing origins.
			resolved, e := filepath.EvalSymlinks(root)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			e = walkDefinitions(resolved, map[string]bool{}, func(path string, d fs.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if d.IsDir() {
					return nil
				}
				if strings.ToLower(filepath.Ext(path)) != ".md" {
					return nil
				}
				if kind == "skills" && filepath.Base(path) != "SKILL.md" {
					return nil
				}
				real, e := filepath.EvalSymlinks(path)
				if e != nil {
					return e
				}
				st, e := os.Stat(real)
				if e != nil {
					return e
				}
				if !st.Mode().IsRegular() {
					return nil
				}
				if st.Size() > 1024*1024 {
					return fmt.Errorf("definition exceeds 1 MiB: %s", path)
				}
				totalBytes += st.Size()
				if totalBytes > 8*1024*1024 {
					return fmt.Errorf("project definition contents exceed 8 MiB")
				}
				b, e := os.ReadFile(real)
				if e != nil {
					return e
				}
				sum := sha256.Sum256(b)
				name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				if strings.EqualFold(name, "SKILL") {
					name = filepath.Base(filepath.Dir(path))
				}
				out = append(out, Definition{Name: name, Kind: kind, Runtime: runtime, Path: real, BaseDir: filepath.Dir(real), Digest: hex.EncodeToString(sum[:]), Content: string(b)})
				if len(out) > 2000 {
					return fmt.Errorf("too many project definitions")
				}
				return nil
			})
			if e != nil {
				return nil, e
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func projectPrompt(target, role, runtime string) (string, error) {
	defs, err := Inventory(target)
	if err != nil {
		return "", err
	}
	return renderProjectPrompt(target, role, runtime, defs)
}
func renderProjectPrompt(target, role, runtime string, defs []Definition) (string, error) {
	var b strings.Builder
	b.WriteString("Project definitions are authoritative for project roles. Resolve relative references from each source directory; do not replace project definitions with generic ancestor roles.\n")
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(target, name)
		data, e := os.ReadFile(path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if len(data) > 1024*1024 {
			return "", fmt.Errorf("project instructions too large")
		}
		if name == "CLAUDE.md" && runtime != "claude" {
			b.WriteString("\nThe following Claude-specific instructions are project context only. Translate intent; do not execute runtime-specific commands unavailable here.\n")
		}
		fmt.Fprintf(&b, "\nProject instructions source %s:\n%s\n", path, data)
	}
	var selected *Definition
	for i := range defs {
		d := &defs[i]
		if d.Kind == "agents" && d.Name == role && (selected == nil || d.Runtime == runtime) {
			selected = d
		}
	}
	if selected != nil {
		fmt.Fprintf(&b, "\nSelected project role %s; source %s; relative references base %s; SHA256 %s:\n%s\n", role, selected.Path, selected.BaseDir, selected.Digest, selected.Content)
	} else {
		fmt.Fprintf(&b, "\nNo project-owned agent definition matched role %q. Use registered instructions and project context; no canonical role was substituted.\n", role)
	}
	b.WriteString("\nAvailable project definitions (read relevant sources before using them):\n")
	for _, d := range defs {
		fmt.Fprintf(&b, "%s %s [%s] %s (base %s; SHA256 %s)\n", d.Kind, d.Name, d.Runtime, d.Path, d.BaseDir, d.Digest)
	}
	if b.Len() > 4*1024*1024 {
		return "", fmt.Errorf("project context exceeds 4 MiB")
	}
	return b.String(), nil
}

// Follow directory links with cycle detection, retaining resolved source paths.
func walkDefinitions(root string, seen map[string]bool, visit fs.WalkDirFunc) error {
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if seen[real] {
		return nil
	}
	seen[real] = true
	if len(seen) > 5000 {
		return fmt.Errorf("definition tree exceeds directory limit")
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(real, entry.Name())
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		if st.IsDir() {
			if err := walkDefinitions(path, seen, visit); err != nil {
				return err
			}
		} else if err := visit(path, entry, nil); err != nil {
			return err
		}
	}
	return nil
}
