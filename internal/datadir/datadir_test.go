package datadir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOf(t *testing.T) {
	home := t.TempDir()
	if got := Of(home); got != filepath.Join(home, ".delistd") {
		t.Errorf("fresh home = %q", got)
	}
	for _, d := range []string{".eraser", ".delistd"} {
		if err := os.Mkdir(filepath.Join(home, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if got := Of(home); got != filepath.Join(home, ".delistd") {
		t.Errorf("both empty = %q", got)
	}
	touch := func(p string) {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	touch(filepath.Join(home, ".eraser", "config.yaml"))
	if got := Of(home); got != filepath.Join(home, ".eraser") {
		t.Errorf("legacy only = %q", got)
	}
	touch(filepath.Join(home, ".delistd", "config.yaml"))
	if got := Of(home); got != filepath.Join(home, ".delistd") {
		t.Errorf("both = %q", got)
	}
}
