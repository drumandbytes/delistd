package fsutil

import "runtime"

// Chmod on Windows can only toggle read-only, and a failure there is not worth
// failing a save for.
func isUnsupported(error) bool { return runtime.GOOS == "windows" }
