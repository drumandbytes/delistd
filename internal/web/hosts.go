package web

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// The Host-header guard against DNS rebinding, and the bind address.

// requireAllowedHost rejects Host headers that are not loopback, private IPs (when listening non-loopback),
// or explicitly allowed (DNS rebinding guard).
func (s *Server) requireAllowedHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			http.Error(w, "Forbidden: this server only accepts requests addressed to localhost or configured allowed hosts", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// listenAddr is the address Start binds: host (default 127.0.0.1) and port.
func listenAddr(host string, port int) string {
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// IsLoopbackListen reports whether host (serve's --listen) binds loopback only.
func IsLoopbackListen(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// hostAllowed reports whether host is permitted by this server.
func (s *Server) hostAllowed(host string) bool {
	return isHostAllowed(host, s.AllowedHosts)
}

// isHostAllowed checks whether host is loopback or matches allowedHosts.
// Patterns in allowedHosts may be:
//   - "*" (allow any Host)
//   - "lan" or "private" (allow any private RFC 1918/4193/link-local/Tailscale IP)
//   - "local" (allow names only a local network can resolve: a single label like
//     "tower", or one under .local, .lan, .home.arpa or .internal)
//   - an IP address (e.g. "192.168.1.100")
//   - a CIDR network (e.g. "10.0.0.0/8", "192.168.0.0/16")
//   - a hostname (e.g. "unraid.local")
//   - a wildcard domain (e.g. "*.local", "*.lan")
func isHostAllowed(host string, allowedHosts []string) bool {
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	h = strings.ToLower(strings.Trim(strings.TrimSpace(h), "[]"))

	switch h {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		return true
	}

	for _, pattern := range allowedHosts {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		if p == "lan" || p == "private" {
			if ip := net.ParseIP(h); ip != nil && isPrivateOrLocalIP(ip) {
				return true
			}
			continue
		}
		if p == "local" {
			if isLocalName(h) {
				return true
			}
			continue
		}
		if ph, _, err := net.SplitHostPort(p); err == nil {
			p = ph
		}
		p = strings.Trim(p, "[]")
		if p == h {
			return true
		}
		if strings.HasPrefix(p, "*.") {
			suffix := strings.TrimPrefix(p, "*")
			if strings.HasSuffix(h, suffix) {
				return true
			}
		}
		if _, cidr, err := net.ParseCIDR(pattern); err == nil {
			if ip := net.ParseIP(h); ip != nil && cidr.Contains(ip) {
				return true
			}
		}
	}

	return false
}

// isLocalName reports whether h is a name only a local network can resolve: a
// single label ("tower") or one under a local-only suffix. DNS rebinding needs a
// public hostname the attacker controls, so none of these can be one.
func isLocalName(h string) bool {
	if h == "" || net.ParseIP(h) != nil {
		return false
	}
	if !strings.Contains(h, ".") {
		return true
	}
	for _, suffix := range []string{".local", ".lan", ".home.arpa", ".internal"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}

// isPrivateOrLocalIP reports whether an IP is loopback, RFC 1918 private,
// link-local, RFC 4193 IPv6 ULA, or RFC 6598 CGNAT / Tailscale (100.64.0.0/10).
func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Check IPv4 Carrier-Grade NAT (RFC 6598: 100.64.0.0/10), used by Tailscale and CGNAT
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && (ip4[1]&0xc0) == 64 {
			return true
		}
	}
	return false
}

// ParseAllowedHosts splits a comma-separated list of host patterns, trimming whitespace.
func ParseAllowedHosts(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var hosts []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			hosts = append(hosts, p)
		}
	}
	return hosts
}
