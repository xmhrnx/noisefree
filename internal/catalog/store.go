package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Store keeps track of the catalog and of what is installed under Dir.
type Store struct {
	Dir string // data folder: engines/<id>/ and catalog/*.json

	mu      sync.Mutex
	entries []Entry
}

// receipt is written next to an installed engine's files.
type receipt struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

// NewStore opens the store in dir, loading user manifests from dir/catalog.
// Manifests that fail to load are reported in the returned error, while the
// rest of the catalog is still usable.
func NewStore(dir string) (*Store, error) {
	s := &Store{Dir: dir}
	err := s.Reload()
	return s, err
}

// Reload rebuilds the catalog from the built-in list and user manifests.
func (s *Store) Reload() error {
	entries := Builtin()
	var errs []error
	files, _ := filepath.Glob(filepath.Join(s.Dir, "catalog", "*.json"))
	sort.Strings(files)
	for _, f := range files {
		e, err := readManifest(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(f), err))
			continue
		}
		entries = upsert(entries, e)
	}
	s.mu.Lock()
	s.entries = entries
	s.mu.Unlock()
	return errors.Join(errs...)
}

func upsert(entries []Entry, e Entry) []Entry {
	for i := range entries {
		if entries[i].ID == e.ID {
			if entries[i].Builtin {
				return entries // manifests can't replace built-in engines
			}
			entries[i] = e
			return entries
		}
	}
	return append(entries, e)
}

func readManifest(path string) (Entry, error) {
	var e Entry
	data, err := os.ReadFile(path)
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return e, err
	}
	e.User, e.Builtin = true, false
	return e, validate(e)
}

func validate(e Entry) error {
	switch {
	case e.ID == "" || strings.ContainsAny(e.ID, `/\.: `):
		return fmt.Errorf("invalid id %q", e.ID)
	case e.Name == "":
		return errors.New("missing name")
	case e.Kind == KindLADSPA && (e.Plugin == nil || e.Plugin.File == ""):
		return errors.New("ladspa engines need plugin.file")
	case e.Kind != KindLADSPA && e.Kind != KindDTLN && e.Kind != KindRuntime:
		return fmt.Errorf("kind %q can't be installed from a manifest", e.Kind)
	}
	return nil
}

// Entries returns a copy of the catalog.
func (s *Store) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entry(nil), s.entries...)
}

// Get returns the entry with id.
func (s *Store) Get(id string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// EngineDir is where an entry's files are installed.
func (s *Store) EngineDir(id string) string { return filepath.Join(s.Dir, "engines", id) }

// Installed reports whether the entry (and everything it requires) is ready.
func (s *Store) Installed(e Entry) bool {
	if e.Builtin {
		return true
	}
	if _, err := os.Stat(filepath.Join(s.EngineDir(e.ID), "receipt.json")); err != nil {
		return false
	}
	for _, r := range e.Requires {
		if re, ok := s.Get(r); !ok || !s.Installed(re) {
			return false
		}
	}
	return true
}

// InstalledVersion returns the version recorded at install time.
func (s *Store) InstalledVersion(id string) string {
	data, err := os.ReadFile(filepath.Join(s.EngineDir(id), "receipt.json"))
	if err != nil {
		return ""
	}
	var r receipt
	_ = json.Unmarshal(data, &r)
	return r.Version
}

// DownloadSizeMB estimates what installing e downloads on this computer,
// including requirements that aren't installed yet.
func (s *Store) DownloadSizeMB(e Entry) int {
	total := 0
	if d, ok := e.DownloadsHere(); ok {
		for _, f := range d {
			total += f.SizeMB
		}
	}
	for _, r := range e.Requires {
		if re, ok := s.Get(r); ok && !s.Installed(re) {
			total += s.DownloadSizeMB(re)
		}
	}
	return total
}

// Remove uninstalls an entry. Shared runtimes are left in place.
func (s *Store) Remove(id string) error {
	e, ok := s.Get(id)
	if !ok {
		return fmt.Errorf("unknown engine %q", id)
	}
	if e.Builtin {
		return errors.New("built-in engines can't be removed")
	}
	if err := os.RemoveAll(s.EngineDir(id)); err != nil {
		return err
	}
	if e.User {
		_ = os.Remove(filepath.Join(s.Dir, "catalog", id+".json"))
		return s.Reload()
	}
	return nil
}

// AddManifest validates a manifest and saves it into the catalog.
func (s *Store) AddManifest(data []byte) (Entry, error) {
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return e, fmt.Errorf("not an engine manifest: %w", err)
	}
	if err := validate(e); err != nil {
		return e, err
	}
	if b, ok := s.Get(e.ID); ok && b.Builtin {
		return e, fmt.Errorf("%q is a built-in engine", e.ID)
	}
	dir := filepath.Join(s.Dir, "catalog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return e, err
	}
	out, _ := json.MarshalIndent(e, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, e.ID+".json"), out, 0o644); err != nil {
		return e, err
	}
	return e, s.Reload()
}

// AddPluginFile installs a LADSPA library the user picked from disk as a
// custom engine and returns its entry.
func (s *Store) AddPluginFile(name string, data []byte) (Entry, error) {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	id := "custom-" + sanitize(base)
	e := Entry{
		ID: id, Name: prettyName(base), Vendor: "Custom plugin", Version: "local", Kind: KindLADSPA,
		Summary:     "LADSPA plugin added from " + filepath.Base(name),
		Description: "A LADSPA effect you installed from a file. NoiseFree runs its first mono effect at 48 kHz; the strength slider blends it with the original signal.",
		Quality:     3, CPUCost: 3, LatencyMs: 10,
		Plugin:    &PluginSpec{File: "plugin"},
		Downloads: map[string][]Download{"*": nil},
	}
	dir := s.EngineDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return e, err
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin"), data, 0o755); err != nil {
		return e, err
	}
	if err := writeReceipt(dir, receipt{Version: "local", Files: []string{"plugin"}}); err != nil {
		return e, err
	}
	return s.AddManifest(mustJSON(e))
}

func writeReceipt(dir string, r receipt) error {
	return os.WriteFile(filepath.Join(dir, "receipt.json"), mustJSON(r), 0o644)
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func prettyName(s string) string {
	s = strings.TrimPrefix(s, "lib")
	s = strings.NewReplacer("_", " ", "-", " ").Replace(s)
	if s == "" {
		return "Custom plugin"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
