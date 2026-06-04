// Package workspace models a becket workspace manifest (.becket.json) and the
// CWD-based workspace detection, matching the bash detect_workspace_id and the
// manifest read/write helpers.
package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/Mykhol/becket/internal/jsonfmt"
)

// RepoEntry is one repo within a workspace manifest (field order: branch, base).
type RepoEntry struct {
	Branch string `json:"branch"`
	Base   string `json:"base"`
}

// Status is the optional status note (field order: text, updatedAt, updatedBy).
type Status struct {
	Text      string `json:"text"`
	UpdatedAt string `json:"updatedAt"`
	UpdatedBy string `json:"updatedBy"`
}

// Manifest is a workspace's .becket.json. Struct field order is the on-disk key
// order ($schema, id, created, description, [stackParent], repos, [status]).
// Repos preserves insertion order via Order (Go maps don't), so the manifest
// lists repos in creation order like the bash JSON does.
type Manifest struct {
	Schema      string               `json:"$schema,omitempty"`
	ID          string               `json:"id"`
	Created     string               `json:"created"`
	Description string               `json:"description"`
	StackParent string               `json:"stackParent,omitempty"`
	Repos       map[string]RepoEntry `json:"repos"`
	Status      *Status              `json:"status,omitempty"`

	// Order records repo insertion order for deterministic marshaling.
	Order []string `json:"-"`
}

// ErrNotInWorkspace is returned by Detect when no manifest is found above CWD.
var ErrNotInWorkspace = errors.New("not inside a becket workspace")

// MarshalJSON emits repos in insertion order (Order), falling back to sorted
// keys, so manifests match the bash dict-order output rather than Go map order.
func (m Manifest) MarshalJSON() ([]byte, error) {
	order := m.Order
	if len(order) == 0 {
		for k := range m.Repos {
			order = append(order, k)
		}
		sort.Strings(order)
	}
	repos := orderedRepos{entries: m.Repos, order: order}

	// Mirror Manifest's field order using an alias to avoid recursion.
	type alias struct {
		Schema      string       `json:"$schema,omitempty"`
		ID          string       `json:"id"`
		Created     string       `json:"created"`
		Description string       `json:"description"`
		StackParent string       `json:"stackParent,omitempty"`
		Repos       orderedRepos `json:"repos"`
		Status      *Status      `json:"status,omitempty"`
	}
	return json.Marshal(alias{
		Schema: m.Schema, ID: m.ID, Created: m.Created, Description: m.Description,
		StackParent: m.StackParent, Repos: repos, Status: m.Status,
	})
}

// orderedRepos marshals a repo map in an explicit key order.
type orderedRepos struct {
	entries map[string]RepoEntry
	order   []string
}

func (o orderedRepos) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}
	for i, k := range o.order {
		if i > 0 {
			buf = append(buf, ',')
		}
		key, _ := json.Marshal(k)
		val, err := json.Marshal(o.entries[k])
		if err != nil {
			return nil, err
		}
		buf = append(buf, key...)
		buf = append(buf, ':')
		buf = append(buf, val...)
	}
	return append(buf, '}'), nil
}

// Load reads and parses a manifest, recovering repo insertion order from the raw
// JSON so re-marshaling preserves it.
func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m.Order = repoKeyOrder(raw)
	return &m, nil
}

// Save writes a manifest as 2-space JSON with a trailing newline.
func Save(path string, m *Manifest) error {
	data, err := jsonfmt.Indent2(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Detect walks up from the current directory looking for a .becket.json and
// returns its id, matching bash detect_workspace_id.
func Detect() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ".becket.json")
		if raw, err := os.ReadFile(candidate); err == nil {
			var m struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(raw, &m) == nil && m.ID != "" {
				return m.ID, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInWorkspace
		}
		dir = parent
	}
}

// Require returns id if non-empty, else detects from CWD.
func Require(id string) (string, error) {
	if id != "" {
		return id, nil
	}
	return Detect()
}

// List returns manifest paths under workspacesDir, sorted (glob order).
func List(workspacesDir string) []string {
	matches, _ := filepath.Glob(filepath.Join(workspacesDir, "*", ".becket.json"))
	sort.Strings(matches)
	return matches
}

// repoKeyOrder extracts the order of keys under "repos" from raw manifest JSON.
func repoKeyOrder(raw []byte) []string {
	return jsonfmt.NestedKeyOrder(raw, "repos")
}
