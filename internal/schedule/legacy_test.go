package schedule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drumandbytes/delistd/internal/testenv"
)

func TestLegacyNoticeIsShownOnceWhileAnOldJobExists(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	for _, c := range []struct {
		os, file string
	}{
		{"darwin", filepath.Join("Library", "LaunchAgents", legacyLaunchdLabel+".plist")},
		{"linux", filepath.Join(".config", "systemd", "user", legacySystemdUnit+".timer")},
	} {
		t.Run(c.os, func(t *testing.T) {
			old := goos
			goos = c.os
			t.Cleanup(func() { goos = old })
			data := t.TempDir()

			if LegacyInstalled() || LegacyNoticeOnce(data) != "" {
				t.Fatal("notice with no old job installed")
			}
			path := filepath.Join(home, c.file)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(path) })

			if !LegacyInstalled() {
				t.Fatal("old job not detected")
			}
			if LegacyNoticeOnce(data) != LegacyNotice {
				t.Error("first call should return the notice")
			}
			if got := LegacyNoticeOnce(data); got != "" {
				t.Errorf("second call = %q, want nothing", got)
			}
		})
	}
}

func TestLegacyNeverTriggersOnOtherPlatforms(t *testing.T) {
	old := goos
	goos = "windows"
	t.Cleanup(func() { goos = old })
	if LegacyInstalled() {
		t.Error("windows has no OS scheduler to leave a legacy job in")
	}
}
