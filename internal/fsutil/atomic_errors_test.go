package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicFailures(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFileAtomic(filepath.Join(dir, "missing", "f"), []byte("x"), 0o600); err == nil {
		t.Error("wrote into a directory that does not exist")
	}

	// The rename onto a non-empty directory fails; the temp file must not be left behind.
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(filepath.Join(target, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(target, []byte("x"), 0o600); err == nil {
		t.Error("replaced a non-empty directory")
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".target.tmp-*"))
	if len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}
