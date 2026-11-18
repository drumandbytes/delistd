package inbox

import (
	"regexp"
	"strings"
)

const addrPattern = `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`

var anyAddress = regexp.MustCompile(addrPattern)

// The wording that introduces the failed address in a bounce notice, most
// specific first. Group 1 is the address.
var recipientCues = func() []*regexp.Regexp {
	cues := []string{
		`(?:the\s+following|these)\s+address(?:es)?\s+(?:had\s+permanent\s+)?(?:fatal\s+)?(?:errors?|failed)[:\s]+`,
		`delivery\s+to\s+(?:the\s+following\s+)?(?:recipient|address)(?:s)?\s+failed[:\s]+`,
		`(?:original|final)[\s-]?recipient[:\s]+(?:rfc822;)?`,
		`(?:failed|rejected)\s+recipient[:\s]+`,
		`undeliverable\s+to[:\s]+`,
		`message\s+could\s+not\s+be\s+delivered\s+to[:\s]+`,
	}
	out := make([]*regexp.Regexp, 0, len(cues)+2)
	for _, c := range cues {
		out = append(out, regexp.MustCompile(`(?i)`+c+`(`+addrPattern+`)`))
	}
	return append(out,
		regexp.MustCompile(`(?i)<(`+addrPattern+`)>.*(?:failed|rejected|undeliverable)`),
		regexp.MustCompile(`(?i)to[:\s]+<?(`+addrPattern+`)>?\s+.*(?:failed|rejected|not\s+exist)`),
	)
}()

// addresses that are never the failed recipient: mail systems, and the
// sender's own mailbox at the big providers
var notTheRecipient = []string{
	"mailer-daemon", "postmaster", "noreply", "no-reply",
	"@gmail.com", "@yahoo.com", "@outlook.com", "@hotmail.com",
}

// ExtractBouncedRecipient returns the address a bounce notice says could not
// be reached, or "" when it cannot tell.
func ExtractBouncedRecipient(email *Email) string {
	text := email.Body
	if email.HTMLBody != "" {
		text += " " + PlainText(email.HTMLBody)
	}
	text += " " + email.Subject

	for _, re := range recipientCues {
		if m := re.FindStringSubmatch(text); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	// no cue matched: the first address that is not a mail system's
	for _, addr := range anyAddress.FindAllString(text, -1) {
		if !containsAny(strings.ToLower(addr), notTheRecipient) {
			return addr
		}
	}
	return ""
}
