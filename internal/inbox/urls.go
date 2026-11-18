package inbox

import (
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// ExtractedURLs are the links in a reply, sorted into the kinds the pipeline
// acts on. A link can be in more than one list; AllURLs has every kept link.
type ExtractedURLs struct {
	FormURLs         []string
	ConfirmationURLs []string
	UnsubscribeURLs  []string
	AllURLs          []string
}

// Substrings of a lowercased URL, and what each is worth as a sign that the
// link leads to an opt-out form.
var (
	formStrong = []string{
		"opt-out", "optout", "opt_out", "do-not-sell", "donotsell", "do_not_sell",
		"removal-request", "removal-form", "removalrequest",
		"remove-my-info", "remove-listing", "remove-record",
		"data-request", "dsar", "data-subject", "ccpa-request", "gdpr-request",
		"privacy-request", "privacy-form",
		"/optout", "/opt-out", "/removal", "/remove-me",
	}
	formModerate = []string{"suppress", "suppression", "ccpa", "gdpr", "/remove", "/delete"}
	// only counted once something stronger matched
	formWeak = []string{"remove", "removal", "delete", "deletion", "unsubscribe"}
	// any of these rules a link out: policy and help pages, logins, account
	// settings, documents, social networks and the usual third-party hosts
	notForm = []string{
		"privacy-policy", "privacy_policy", "privacypolicy",
		"terms-of-service", "terms_of_service", "termsofservice",
		"terms-and-conditions", "terms_and_conditions",
		"cookie-policy", "cookie_policy", "cookiepolicy",
		"/tos", "/terms", "/legal", "/policy",
		"/about", "/contact", "/help", "/faq", "/support", "/how-to", "/howto", "/learn", "/info",
		"/login", "/signin", "/register", "/signup", "/auth",
		"/account", "/settings", "/preferences", "/profile",
		"/unsubscribe-preferences", "/email-preferences", "/manage-preferences", "/communication-preferences",
		"/marketing", "/newsletter", "/subscribe",
		".pdf", ".doc", ".docx",
		"facebook.com", "twitter.com", "linkedin.com", "instagram.com",
		"google.com", "bit.ly", "tinyurl.com",
	}
	confirmHints  = []string{"confirm", "verification", "verify", "activate", "validate", "click-here", "clickhere", "token=", "code=", "approve", "accept"}
	trackingHints = []string{"track", "pixel", "beacon", "open.gif", "spacer.gif", "1x1", "unsubscribe-tracking"}
)

const (
	notFormScore  = -20
	sameSiteBonus = 20
)

var urlInText = regexp.MustCompile(`https?://[^\s<>"']+`)

func containsAny(s string, hints []string) bool {
	return slices.ContainsFunc(hints, func(h string) bool { return strings.Contains(s, h) })
}

func countIn(s string, hints []string) int {
	n := 0
	for _, h := range hints {
		if strings.Contains(s, h) {
			n++
		}
	}
	return n
}

// ParseEmailURLs collects the links from both bodies, drops tracking pixels
// and anything pointing at a private address, and sorts the rest.
func ParseEmailURLs(email *Email) ExtractedURLs {
	var found []string
	if email.Body != "" {
		found = urlInText.FindAllString(email.Body, -1)
	}
	if email.HTMLBody != "" {
		found = append(found, linksInHTML(email.HTMLBody)...)
	}

	var out ExtractedURLs
	seen := map[string]bool{}
	for _, raw := range found {
		u := normalizeURL(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		lower := strings.ToLower(u)
		if isTracker(lower) {
			continue
		}
		out.AllURLs = append(out.AllURLs, u)
		if formScore(lower) > 0 {
			out.FormURLs = append(out.FormURLs, u)
		}
		if containsAny(lower, confirmHints) {
			out.ConfirmationURLs = append(out.ConfirmationURLs, u)
		}
		if strings.Contains(lower, "unsubscribe") {
			out.UnsubscribeURLs = append(out.UnsubscribeURLs, u)
		}
	}
	return out
}

// linksInHTML returns every <a href>, then any bare URLs in the visible text.
// The size limit is applied where the message is read.
func linksInHTML(doc string) []string {
	var links []string
	var text strings.Builder
	z := html.NewTokenizer(strings.NewReader(doc))
	for {
		switch z.Next() {
		case html.ErrorToken:
			// end of input, or markup too broken to go on
			return append(links, urlInText.FindAllString(text.String(), -1)...)
		case html.TextToken:
			text.Write(z.Text())
			text.WriteByte(' ')
		case html.StartTagToken, html.SelfClosingTagToken:
			name, more := z.TagName()
			if string(name) != "a" {
				continue
			}
			for more {
				var k, v []byte
				k, v, more = z.TagAttr()
				if string(k) == "href" {
					links = append(links, string(v))
				}
			}
		}
	}
}

// normalizeURL trims trailing punctuation and returns the URL re-serialised,
// or "" for anything that is not an http(s) link to a public host. Emails are
// attacker-influenced, so private targets (cloud metadata at 169.254.169.254,
// say) are refused even when domain validation is off.
func normalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimRight(raw, ".,;:!?)"))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	if privateHost(u.Hostname()) {
		return ""
	}
	return u.String()
}

// privateHost is true for localhost and for literal loopback, private,
// link-local and unspecified addresses. Names are not resolved; the broker
// domain allowlist covers those.
func privateHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified())
}

// formScore says how likely a lowercased URL is an opt-out form: above zero is
// plausible, notFormScore is ruled out.
func formScore(lower string) int {
	if containsAny(lower, notForm) {
		return notFormScore
	}
	score := 10*countIn(lower, formStrong) + 5*countIn(lower, formModerate)
	if score > 0 {
		score += 2 * countIn(lower, formWeak)
	}
	return score
}

func isTracker(lower string) bool {
	return containsAny(lower, trackingHints) ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".png") && strings.Contains(lower, "pixel")
}

// GetPrimaryFormURL picks the likeliest opt-out form: the best-scoring form
// link, with a bonus for one on the broker's own domain. Ties go to the link
// that came first.
func GetPrimaryFormURL(urls ExtractedURLs, brokerDomain string) string {
	domain := strings.ToLower(brokerDomain)
	best, bestScore := "", 0
	for _, u := range urls.FormURLs {
		lower := strings.ToLower(u)
		score := formScore(lower)
		if score < 0 {
			continue
		}
		if domain != "" && strings.Contains(lower, domain) {
			score += sameSiteBonus
		}
		if best == "" || score > bestScore {
			best, bestScore = u, score
		}
	}
	return best
}

// GetPrimaryConfirmationURL prefers a confirmation link on the broker's domain
// and otherwise takes the first one.
func GetPrimaryConfirmationURL(urls ExtractedURLs, brokerDomain string) string {
	for _, u := range urls.ConfirmationURLs {
		if strings.Contains(strings.ToLower(u), brokerDomain) {
			return u
		}
	}
	if len(urls.ConfirmationURLs) > 0 {
		return urls.ConfirmationURLs[0]
	}
	return ""
}
