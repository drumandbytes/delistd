package inbox

import (
	"regexp"
	"strings"
)

// A subject-line hit is a stronger signal than a phrase in the body.
const subjectPoints = 3

// A URL of the right kind in the reply backs up a form or confirmation verdict.
const urlPoints = 2

// scored lists the types that compete on points. On a tie the earlier one
// wins, which puts the types that keep a reply in front of a person (a form
// to fill, a link to click) ahead of the terminal ones that close it out.
var scored = [...]ResponseType{
	ResponseConfirmationRequired,
	ResponseFormRequired,
	ResponsePending,
	ResponseRejected,
	ResponseSuccess,
}

type points map[ResponseType]int

func count(list []*regexp.Regexp, texts ...string) int {
	n := 0
	for _, re := range list {
		for _, t := range texts {
			if re.MatchString(t) {
				n++
				break
			}
		}
	}
	return n
}

// subjectPoints scores the subject-only phrase lists.
func (p points) addSubject(subject string) {
	p[ResponsePending] += subjectPoints * count(subjectPendingPatterns, subject)
	p[ResponseRejected] += subjectPoints * count(subjectRejectionPatterns, subject)
	p[ResponseSuccess] += subjectPoints * count(subjectSuccessPatterns, subject)
	p[ResponseFormRequired] += subjectPoints * count(subjectFormPatterns, subject)
}

// top returns the winning type, its points and the runner-up's points
// (equal to the winner's on a tie). Nothing scored yields "" and 0.
func (p points) top() (ResponseType, int, int) {
	var best ResponseType
	first, second := 0, 0
	for _, t := range scored {
		switch s := p[t]; {
		case s > first:
			best, first, second = t, s, first
		case s > second:
			second = s
		}
	}
	return best, first, second
}

// emailText is the lowercase text the phrases run against.
func emailText(e *Email) (subject, content string) {
	content = e.Body
	if content == "" {
		content = PlainText(e.HTMLBody)
	}
	return strings.ToLower(e.Subject), strings.ToLower(content)
}

// scoreReply awards points to each type for the phrases and links in a reply.
func scoreReply(subject, content string, urls ExtractedURLs) points {
	p := points{}
	p.addSubject(subject)
	p[ResponseSuccess] += count(successPatterns, content, subject)
	p[ResponseFormRequired] += count(formRequiredPatterns, content)
	p[ResponseConfirmationRequired] += count(confirmationPatterns, content)
	p[ResponseRejected] += count(rejectionPatterns, content, subject)
	p[ResponsePending] += count(pendingPatterns, content)
	if len(urls.FormURLs) > 0 {
		p[ResponseFormRequired] += urlPoints
	}
	if len(urls.ConfirmationURLs) > 0 {
		p[ResponseConfirmationRequired] += urlPoints
	}
	return p
}

// ClassifyResponse decides what a reply from a broker amounts to.
func ClassifyResponse(email *Email) ClassifiedResponse {
	res := ClassifiedResponse{Email: email, Type: ResponseUnknown, URLs: ParseEmailURLs(email)}
	subject, content := emailText(email)

	if anyPattern(testEmailPatterns, subject) || anyPattern(testEmailPatterns, content) {
		res.Type, res.Confidence = ResponseSuccess, 1.0
		res.Reason = "Test email from Delistd - email configuration verified"
		return res
	}

	if isBounce(email, subject, content) {
		res.Type, res.Confidence = ResponseBounced, 0.95
		res.Reason = "Email delivery failed - address may be invalid"
		res.BouncedRecipient = ExtractBouncedRecipient(email)
		return res
	}

	if matched, review := noDataVerdict(subject, emailBody(email)); matched {
		res.Type, res.Confidence, res.NeedsReview = ResponseNoData, 0.85, review
		if review {
			res.Confidence = 0.5
			res.FormURL = GetPrimaryFormURL(res.URLs, email.FromDomain)
			res.ConfirmURL = GetPrimaryConfirmationURL(res.URLs, email.FromDomain)
		}
		res.Reason = reasonFor(res.Type)
		return res
	}

	p := scoreReply(subject, content, res.URLs)
	best, top, runnerUp := p.top()
	switch {
	case top > 0:
		res.Type = best
		res.Confidence = confidenceFor(top, runnerUp)
		if top >= subjectPoints {
			res.Confidence = max(res.Confidence, 0.75)
		}
		if best == ResponseFormRequired && len(res.URLs.FormURLs) > 0 ||
			best == ResponseConfirmationRequired && len(res.URLs.ConfirmationURLs) > 0 {
			res.Confidence = max(res.Confidence, 0.85)
		}
		// two types on equal points is a coin toss the next reader should see
		res.NeedsReview = top == runnerUp
	case len(res.URLs.ConfirmationURLs) > 0:
		res.Type, res.Confidence = ResponseConfirmationRequired, 0.5
	case len(res.URLs.FormURLs) > 0:
		res.Type, res.Confidence = ResponseFormRequired, 0.5
	}

	res.FormURL = GetPrimaryFormURL(res.URLs, email.FromDomain)
	res.ConfirmURL = GetPrimaryConfirmationURL(res.URLs, email.FromDomain)
	res.Reason = reasonFor(res.Type)

	// `rejected` and `success` take a broker off the overdue list that feeds
	// an export or a DPA complaint, so they need more certainty than the rest.
	terminal := res.Type == ResponseRejected || res.Type == ResponseSuccess
	res.NeedsReview = res.NeedsReview || res.Type == ResponseUnknown ||
		res.Confidence < 0.4 || (terminal && res.Confidence < 0.7)
	return res
}

// confidenceFor is 0.85 when only one type scored, else 0.5-0.9 by how far
// the winner is ahead.
func confidenceFor(top, runnerUp int) float64 {
	if runnerUp == 0 {
		return 0.85
	}
	margin := float64(top-runnerUp) / float64(top)
	return 0.5 + margin*0.4
}

// ClassifyBySubjectOnly classifies from a subject line alone, for records
// whose body was not kept.
func ClassifyBySubjectOnly(subject string) (ResponseType, float64, bool) {
	if matched, review := noDataVerdict(subject, ""); matched {
		if review {
			return ResponseNoData, 0.5, true
		}
		return ResponseNoData, 0.7, false
	}
	subject = strings.ToLower(subject)

	p := points{}
	p.addSubject(subject)
	p[ResponseSuccess] += count(successPatterns, subject)
	p[ResponseRejected] += count(rejectionPatterns, subject)
	p[ResponsePending] += count(pendingPatterns, subject)

	best, top, _ := p.top()
	switch {
	case top >= subjectPoints:
		return best, 0.7, false
	case top >= 1:
		return best, 0.4, true
	}
	return ResponseUnknown, 0, true
}

// isBounce is true for a mail-system sender with any bounce wording, or for
// wording strong enough on its own.
func isBounce(e *Email, subject, content string) bool {
	from, name := strings.ToLower(e.From), strings.ToLower(e.FromName)
	fromMailSystem := false
	for _, s := range bounceSenders {
		if strings.Contains(from, s) || strings.Contains(name, s) {
			fromMailSystem = true
			break
		}
	}
	score := 0
	for _, re := range bouncePatterns {
		if re.MatchString(subject) {
			score += 2
		}
		if re.MatchString(content) {
			score++
		}
	}
	return fromMailSystem && score > 0 || score >= 3
}
