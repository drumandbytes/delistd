// Package fsutil has the small file helpers the rest of the program shares.
package fsutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data in one step: the data goes to a
// temporary file in the same directory, is flushed to disk, and is renamed
// over the target, so a crash, a full disk or a power cut leaves either the
// old file or the new one, never a truncated config. A symlink at path is
// followed (the file it points to is replaced) instead of being swapped for a
// regular file, which keeps dotfile-managed setups intact. A read-only target
// is refused, as a plain write would.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	// a file made read-only is meant to stay as it is; a rename would
	// replace it regardless, so honour the mode the way a plain write would
	if fi, err := os.Stat(path); err == nil && fi.Mode().Perm()&0o200 == 0 {
		return &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(perm); err != nil && !isUnsupported(err) {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
