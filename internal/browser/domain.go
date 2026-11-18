package browser

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// matchesAllowedDomain reports whether rawURL's host is an allowed domain or a
// subdomain of one. Shared by ValidateDomain and NavigateAndFill.
func matchesAllowedDomain(rawURL string, allowedDomains []string) (bool, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, "", fmt.Errorf("invalid URL: %w", err)
	}

	// Hostname drops the port and the brackets of an IPv6 literal; a trailing
	// dot names the same host
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")

	for _, d := range allowedDomains {
		d = strings.ToLower(d)
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true, d, nil
		}
	}
	return false, host, nil
}

func containsAny(text string, phrases []string) bool {
	return slices.ContainsFunc(phrases, func(p string) bool { return strings.Contains(text, p) })
}
