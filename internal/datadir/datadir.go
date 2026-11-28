// Package datadir resolves where Delistd keeps config.yaml, history.db and
// friends.
package datadir

import (
	"os"
	"path/filepath"
)

// Of returns ~/.delistd, unless the pre-rename ~/.eraser holds a config and
// ~/.delistd does not, which keeps existing setups (and Docker volumes still
// mounted at the old path) working without moving any data.
func Of(home string) string {
	next := filepath.Join(home, ".delistd")
	if has(next) {
		return next
	}
	if legacy := filepath.Join(home, ".eraser"); has(legacy) {
		return legacy
	}
	return next
}

func has(dir string) bool {
	for _, f := range []string{"config.yaml", "history.db"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}
