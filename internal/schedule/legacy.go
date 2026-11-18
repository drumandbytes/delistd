package schedule

import (
	"os"
	"path/filepath"
)

// Everything about the pre-3.0 "eraser" job notice lives in this file, so it
// can be deleted whole once nobody upgrades from before 2.4 any more. (Also
// remove the legacy names and removeLegacy in os.go, the call in cmd/delistd's
// root command, and the Automation card's legacy line.)

// LegacyNotice is what to tell someone whose OS job still runs `eraser`.
const LegacyNotice = "An OS schedule installed under the old Eraser name is still set up, but the `eraser` command is gone in 3.0, so it no longer runs. Replace it with `delistd schedule install`."

// LegacyInstalled reports whether a job is installed under the pre-3.0 names.
func LegacyInstalled() bool {
	var path string
	switch goos {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		path = filepath.Join(home, "Library", "LaunchAgents", legacyLaunchdLabel+".plist")
	case "linux":
		dir, err := systemdDir()
		if err != nil {
			return false
		}
		path = filepath.Join(dir, legacySystemdUnit+".timer")
	default:
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

const legacyMarker = "legacy-schedule-notice"

// LegacyNoticeOnce returns LegacyNotice the first time it is asked while a
// legacy job exists, and "" afterwards: it leaves a marker file in dataDir so
// the CLI says it once instead of on every command.
func LegacyNoticeOnce(dataDir string) string {
	marker := filepath.Join(dataDir, legacyMarker)
	if !LegacyInstalled() {
		return ""
	}
	if _, err := os.Stat(marker); err == nil {
		return ""
	}
	_ = os.WriteFile(marker, nil, 0o600)
	return LegacyNotice
}
