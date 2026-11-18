package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomicReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new contents" {
		t.Errorf("file = %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only the file: %v", len(entries), entries)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
		}
	}
}

func TestWriteFileAtomicCreatesAndFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.yaml")
	if err := WriteFileAtomic(fresh, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return // creating symlinks needs privileges there
	}
	target := filepath.Join(dir, "managed.yaml")
	link := filepath.Join(dir, "link.yaml")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("via link"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file")
	}
	if got, _ := os.ReadFile(target); string(got) != "via link" {
		t.Errorf("target = %q", got)
	}
}

func TestWriteFileAtomicFailsCleanlyWithoutADirectory(t *testing.T) {
	if err := WriteFileAtomic(filepath.Join(t.TempDir(), "missing", "f"), []byte("x"), 0o600); err == nil {
		t.Error("want an error when the directory does not exist")
	}
}

func TestWriteFileAtomicRefusesAReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only is an attribute there")
	}
	path := filepath.Join(t.TempDir(), "locked.yaml")
	if err := os.WriteFile(path, []byte("keep"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); !os.IsPermission(err) {
		t.Errorf("err = %v, want a permission error", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "keep" {
		t.Errorf("a read-only file was replaced: %q", got)
	}
}
