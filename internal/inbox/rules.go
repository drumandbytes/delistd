package inbox

import (
	"regexp"
	"strings"
)

// Phrase lists the classifier scores against. In a phrase a space stands for
// any run of whitespace and an underscore for optional whitespace; matching is
// case-insensitive. Each phrase that matches adds one point to its list's
// category, so splitting or merging phrases changes scores: the golden test
// in classifier_golden_test.go pins that.
func phrases(list ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(list))
	for i, p := range list {
		p = strings.ReplaceAll(p, " ", `\s+`)
		p = strings.ReplaceAll(p, "_", `\s*`)
		out[i] = regexp.MustCompile("(?i)" + p)
	}
	return out
}

var (
	// removal done (body or subject)
	successPatterns = phrases(
		`request (has been )?(completed|processed|fulfilled)`,
		`successfully (removed|deleted|opted_out)`,
		`your (data|information) (has been )?(removed|deleted)`,
		`opt[\s-]?out (request )?(is )?(complete|confirmed)`,
		`we have (removed|deleted)`,
		`no longer (have|hold|store) your (data|information)`,
		// replies relayed by privacy services (Retention.com, DataGrail, TrustSuperset);
		// done tense only, a promise ("will be deleted within 45 days") is not a success
		`(have|has) (now )?processed your opt[\s-]?out`,
		`(has|have) been marked for removal`,
		`(data|information|records?|profile) (has|have) been permanently (deleted|removed)`,
		`request to exercise your [\w\s-]{0,60}? has been completed`,
	)

	// the broker wants a web form instead of email
	formRequiredPatterns = phrases(
		`please (complete|fill_(out|in)?|submit) (the|our|this)?_(form|request)`,
		`visit (the )?(following )?(link|url|page) to (complete|submit|verify)`,
		`click (here|below|the link) to (begin|start|submit|complete)`,
		`(must|need to) (verify|confirm) your (identity|request)`,
		`submit a (formal )?request (through|via|at)`,
		`use (our|the) (online|web)_(form|portal|tool)`,
		`please submit (your )?request (at|via|through) `,
		`please use (our|the) (opt[\s-]?out|removal|privacy)_(form|page|link)`,
		`complete (the|our|a) (data subject|privacy|opt[\s-]?out)_(access )?(request )?form`,
		`submit (a |your )?(request|form) (via|through|at|using) (our )?(online|web|interactive)`,
		`(does not|do not|cannot) accept privacy requests? (via|by|through) email`,
		`this email (address )?is not intended for privacy`,
		`visit (our|the) (opt[\s-]?out|removal|privacy)_(page|form|portal)`,
		`(data subject|privacy) requests? (can|should|must) be (filed|submitted) (at|via)`,
		`go to (the )?(link|url|page) (below|above)`,
		`please click (on )?(the )?following link`,
		`you (can|may) submit .{0,30}(privacy|opt[\s-]?out)`,
		`right to (opt[\s-]?out|delete|know)[:\s]`,
		`we (have )?established a dedicated (online )?form`,
		`we do not process requests? via email`,
		`please send (your? )?request to customer service`,
		`is not (a )?mechanism for.{0,30}(privacy|request)`,
		`please complete your (request|form)`,
	)

	// a link or reply is needed to confirm identity or address
	confirmationPatterns = phrases(
		`click (here|below|the link) to (confirm|verify|validate)`,
		`please (confirm|verify) (your )?(email|request|identity)`,
		`verification (link|email|code)`,
		`confirm your (email )?(address)?`,
		`click (to )?confirm`,
		`(can|could) you (please )?verify`,
		`verify (your )?(last 4|ssn|social)`,
	)

	// the broker refuses, is exempt, or is the wrong address
	rejectionPatterns = phrases(
		`(cannot|can't|unable to) (process|complete|fulfill) (your )?request`,
		`request (has been )?(denied|rejected|declined)`,
		`exempt from (CCPA|GDPR|this request)`,
		`no longer (registered|operating) (as )?(an? )?(active )?data broker`,
		`(this )?(email|inbox)( \w+)? (is )?(no longer|being retired)`,
		`service offerings? no longer include`,
		`(we are|we're) a b2b (platform|company|business)`,
		`consumer reporting agenc(y|ies) (is|are) exempt`,
		`fair credit reporting act.{0,30}exempt`,
		`we do not (remove|delete) data by request`,
		`(this )?(email|inbox) (address )?(is )?not (a )?(mechanism|intended) (for|to)`,
		`not intended for (the )?(submission|handling) of privacy`,
		`will not be considered a valid submission`,
	)

	// acknowledgement, ticket, auto-reply, or "we will follow up"
	pendingPatterns = phrases(
		`(is being|currently being) (processed|reviewed|handled)`,
		`will (process|complete|handle) (your )?request within`,
		`please allow (\d+) (days|business days|weeks)`,
		`we('ll| will) (get back|respond|follow up)`,
		`request (has been )?(received|acknowledged)`,
		`thank you for (your )?(inquiry|email|contacting|reaching|privacy)`,
		`we (have )?received your ([\w-]+ )?(request|email|inquiry)`, // "received your Deletion request"
		`(receive|get) (another|a further) (email|message|notification|update)`,
		`once (we have|we've|it has been|your request has been) (processed|completed)`,
		`(has been )?assigned (a )?(ticket|case|reference)_(number|#|id)?`,
		`one of our .{0,30}(will )?(reach out|respond|contact)`,
		`ticket (has been )?(created|opened|received)`,
		`your request has been received`,
		`support request_#?\d+`,
		`legal request received`,
		`i (have )?(now )?left `,
		`no longer with (the )?(company|organization)`,
		`(will be )?(removed|deleted) from our database.{0,20}\d+ days`,
		`once verified.{0,30}(will be )?(processed|complete)`,
		`this (message )?confirms (our )?receipt`,
		`we appreciate your interest in exercising`,
		`request (will be )?(processed|fulfilled)`,
		`automatic reply`,
		`auto[\s-]?response`,
	)

	// completion wording in a subject line
	subjectSuccessPatterns = phrases(
		`opt[\s-]?out (has been )?completed`,
		`(has been |successfully )?(removed|deleted)`,
		`ticket.+solved`,
		`request (has been )?(completed|processed|fulfilled)`,
	)

	// "how to opt out" style subject lines
	subjectFormPatterns = phrases(
		`opt[\s-]?out instructions`,
		`removal instructions`,
		`how to (opt[\s-]?out|remove)`,
	)

	// refusal wording in a subject line
	subjectRejectionPatterns = phrases(
		`unable to process`,
		`request (denied|rejected)`,
	)

	// acknowledgement wording that is a strong signal in a subject line
	subjectPendingPatterns = phrases(
		`^automatic reply`,
		`^auto[\s-]?reply`,
		`^auto[\s-]?response`,
		`^out of office`,
		`request received`,
		`has been received`,
		`thank you for your (privacy|data|removal|email)`,
		`thank you for (your )?email to`,
		`thanks for (reaching|contacting)`,
		`#[A-Z]{0,3}[-]?\d{5,}`,
		`request_#_\d+`,
		`support request`,
		`ticket_[\(#]_:?_\d+`,
		`we have received your ticket`,
		`i (have )?(now )?left `,
		`no longer with (the )?(company|organization)`,
		`office closed`,
		`response to your email`,
	)

	// Delistd's own test email, never a broker reply
	testEmailPatterns = phrases(
		`delistd test email`,
		`test email from delistd`,
		`this is a test email`,
	)

	// delivery failure notices
	bouncePatterns = phrases(
		`delivery (to .+ )?(has )?failed`,
		`undeliverable`,
		`delivery status notification`,
		`returned mail`,
		`mail delivery failed`,
		`message (could )?not (be )?delivered`,
		`could not be delivered`,
		`delivery failure`,
		`permanent (failure|error)`,
		`address rejected`,
		`user unknown`,
		`mailbox not found`,
		`no such user`,
		`(mailbox|recipient|address) (does not|doesn't) exist`,
		`invalid (recipient|address|mailbox)`,
		`unknown (recipient|user|address)`,
		`550 .* (rejected|unknown|not found)`,
		`554 .* (rejected|failed)`,
	)

	// senders that only ever write bounce notices
	bounceSenders = []string{"mailer-daemon", "mailerdaemon", "postmaster", "mail delivery system", "mail delivery subsystem", "mailsystem", "noreply", "no-reply"}
)
