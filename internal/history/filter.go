package history

import "time"

// MatchesFilter is the one definition of the broker status filters the CLI's
// `send --status` and the web brokers page share. st is nil for a broker never
// contacted. "pending" is the web UI's word for "never"; an unknown filter
// matches everything, like "all".
func MatchesFilter(filter string, st *BrokerStatus, now time.Time) bool {
	switch filter {
	case "never", "pending":
		return st == nil
	case "sent":
		return st != nil && st.Status == StatusSent
	case "failed":
		return st != nil && st.Status == StatusFailed
	case "eligible":
		return st == nil || st.Status != StatusSent || now.Sub(st.LastSent) >= ResendCooldown
	}
	return true
}

// IsSendFilter reports whether s names a status filter a send may use:
// `send --status` and the web "Send all" accept only these four.
func IsSendFilter(s string) bool {
	switch s {
	case "eligible", "never", "failed", "all":
		return true
	}
	return false
}
