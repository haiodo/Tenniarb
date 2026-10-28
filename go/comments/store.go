package comments

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Environment variables that point the application and the service at another config folder; the same
// ones the app reads, so both find the same files.
const (
	EnvConfigDir = "TENNIARB_CONFIG_DIR"
	EnvQAData    = "GOWT_QA_DATA"
)

// ConfigDir is the user's Tenniarb folder: settings, window frames and comments live here.
func ConfigDir() string {
	for _, env := range []string{EnvConfigDir, EnvQAData} {
		if d := os.Getenv(env); d != "" {
			return d
		}
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "Tenniarb")
	}
	return filepath.Join(os.TempDir(), "Tenniarb")
}

// DirIn is the folder of the comment files in the config folder config.
func DirIn(config string) string { return filepath.Join(config, "comments") }

// Store keeps the comments of every document in dir, one file each. Writes go through one Store of the
// service; a reader in another process may read the files at any time, as a write replaces a file whole.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore returns a store in dir; nothing is created before the first write.
func NewStore(dir string) *Store { return &Store{dir: dir} }

// Dir is the folder of the files.
func (s *Store) Dir() string { return s.dir }

func (s *Store) file(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:8])+".json")
}

// Load reads the comments of the document key; a document without a file has none.
func (s *Store) Load(key string) (Doc, error) {
	data, err := os.ReadFile(s.file(key))
	if errors.Is(err, fs.ErrNotExist) {
		return Doc{V: Version, Key: key}, nil
	}
	if err != nil {
		return Doc{}, err
	}
	var d Doc
	if err := json.Unmarshal(data, &d); err != nil {
		return Doc{}, fmt.Errorf("comments of %s: %v", key, err)
	}
	if d.Key != key {
		return Doc{}, fmt.Errorf("comments file of %s holds %s", key, d.Key)
	}
	return d, nil
}

// Update reads the comments of key, lets f change them and writes the result with a higher Rev. When f
// fails nothing is written. A file that cannot be read is an error, never overwritten.
func (s *Store) Update(key string, f func(*Doc) error) (Doc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.Load(key)
	if err != nil {
		return Doc{}, err
	}
	if err := f(&d); err != nil {
		return Doc{}, err
	}
	d.V, d.Key = Version, key
	d.Rev++
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return Doc{}, err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return Doc{}, err
	}
	return d, writeAtomic(s.file(key), data)
}

// writeAtomic writes path through a temporary file in the same folder, so a reader sees the old file or
// the new one whole.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil && runtime.GOOS != "windows" {
		err = os.Chmod(name, 0o600)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}
