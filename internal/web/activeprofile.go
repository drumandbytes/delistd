package web

import (
	"net/http"
	"slices"

	"github.com/drumandbytes/delistd/internal/config"
)

// activeProfileCookie holds the web UI's profile choice. It is neither HttpOnly
// nor Secure: it carries no secret, and a value that names no configured
// profile falls back to the first one.
const activeProfileCookie = "delistd_profile"

// activeProfile is the request's profile: the one the cookie names, else the
// first configured. Unlike the CLI's --profile it is never ambiguous. Handlers
// use this, never the config's profile directly.
func (s *Server) activeProfile(r *http.Request) config.NamedProfile {
	cfg := s.getConfig()
	if cfg == nil {
		return config.NamedProfile{ID: config.DefaultProfileID}
	}
	profiles := cfg.GetProfiles()
	if c, err := r.Cookie(activeProfileCookie); err == nil {
		if i := slices.IndexFunc(profiles, func(p config.NamedProfile) bool { return p.ID == c.Value }); i >= 0 {
			return profiles[i]
		}
	}
	return profiles[0]
}
