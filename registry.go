// registry.go
package goplt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// RegistryEntry is one registered template-collection root.
type RegistryEntry struct {
	Module string `toml:"module"`
}

// Registry is the deserialised form of $XDG_CONFIG_HOME/goplt/registry.toml.
// The zero value is a valid, empty registry.
type Registry struct {
	Collections []RegistryEntry `toml:"collections"`
}

// Has reports whether module is already registered (exact-match).
func (r Registry) Has(module string) bool {
	for _, e := range r.Collections {
		if e.Module == module {
			return true
		}
	}

	return false
}

// ValidateRegistryModule enforces the v1 rules for a registered module path:
// non-empty, no local-path prefix, no @version, hostname must contain a dot.
func ValidateRegistryModule(module string) error {
	if module == "" {
		return errors.New("module path is empty")
	}

	if module[0] == '.' || module[0] == '/' || module[0] == '~' {
		return fmt.Errorf("module path %q looks local; expected a remote Go module reference", module)
	}

	if strings.Contains(module, "@") {
		return fmt.Errorf("module path %q must not contain @version; refresh always uses @latest", module)
	}

	host := strings.SplitN(module, "/", 2)[0]
	if !strings.Contains(host, ".") {
		return fmt.Errorf("module path %q: first segment %q is not a hostname (missing dot)", module, host)
	}

	return nil
}

// Add returns a new Registry with module appended. It validates the module path
// and rejects duplicates. The receiver is not mutated.
func (r Registry) Add(module string) (Registry, error) {
	if err := ValidateRegistryModule(module); err != nil {
		return r, fmt.Errorf("validate module: %w", err)
	}

	if r.Has(module) {
		return r, fmt.Errorf("module %q is already registered", module)
	}

	out := Registry{Collections: make([]RegistryEntry, len(r.Collections), len(r.Collections)+1)}
	copy(out.Collections, r.Collections)
	out.Collections = append(out.Collections, RegistryEntry{Module: module})

	return out, nil
}

// Remove returns a new Registry with module removed. It errors when module is
// not present. The receiver is not mutated.
func (r Registry) Remove(module string) (Registry, error) {
	for i, e := range r.Collections {
		if e.Module != module {
			continue
		}

		out := Registry{Collections: make([]RegistryEntry, 0, len(r.Collections)-1)}
		out.Collections = append(out.Collections, r.Collections[:i]...)
		out.Collections = append(out.Collections, r.Collections[i+1:]...)

		return out, nil
	}

	return r, fmt.Errorf("module %q is not registered", module)
}

// RegistryStore persists a Registry to a TOML file.
// The zero value is not usable; construct via DefaultRegistryStore or NewRegistryStore.
type RegistryStore struct {
	path string
}

// NewRegistryStore returns a store that reads/writes at path.
// It is useful for tests and callers that want a non-default location.
func NewRegistryStore(path string) *RegistryStore {
	return &RegistryStore{path: path}
}

// Path returns the on-disk location this store reads from and writes to.
func (s *RegistryStore) Path() string {
	return s.path
}

// Load reads and parses the registry file. A missing file returns an empty
// Registry and nil error. Parse errors are wrapped.
func (s *RegistryStore) Load() (Registry, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Registry{}, nil
		}

		return Registry{}, fmt.Errorf("read registry %q: %w", s.path, err)
	}

	var r Registry
	if err := toml.Unmarshal(data, &r); err != nil {
		return Registry{}, fmt.Errorf("parse registry %q: %w", s.path, err)
	}

	return r, nil
}

// Save writes the registry to disk, creating the parent directory if needed.
// File permission is 0o644; directory permission is 0o755.
func (s *RegistryStore) Save(r Registry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	data, err := toml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write registry %q: %w", s.path, err)
	}

	return nil
}

// DefaultRegistryStore returns a RegistryStore pointed at
// $XDG_CONFIG_HOME/goplt/registry.toml on Linux,
// ~/Library/Application Support/goplt/registry.toml on macOS,
// %AppData%\goplt\registry.toml on Windows.
func DefaultRegistryStore() (*RegistryStore, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate user config dir: %w", err)
	}

	return NewRegistryStore(filepath.Join(cfg, "goplt", "registry.toml")), nil
}
