package web

import (
	"os/exec"
	"runtime"
)

// OpenBrowser opens url in the default browser. Tests replace it.
var OpenBrowser = func(url string) {
	if name, args := browserCommand(runtime.GOOS, url); name != "" {
		_ = exec.Command(name, args...).Start()
	}
}

// browserCommand is the command line that opens url on goos, or "" where
// there is none.
func browserCommand(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "linux":
		return "xdg-open", []string{url}
	case "windows":
		return "cmd", []string{"/c", "start", url}
	}
	return "", nil
}
