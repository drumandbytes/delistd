package inbox

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emersion/go-imap"

	"github.com/drumandbytes/delistd/internal/broker"
)

func messageWith(raw string) (*imap.Message, *imap.BodySectionName) {
	section := &imap.BodySectionName{}
	return &imap.Message{
		Uid: 7,
		Envelope: &imap.Envelope{
			Subject:   "Re: your request",
			MessageId: "<abc@broker.example>",
			From:      []*imap.Address{{PersonalName: "Privacy Team", MailboxName: "privacy", HostName: "Broker.Example"}},
		},
		Body: map[*imap.BodySectionName]imap.Literal{section: bytes.NewReader([]byte(raw))},
	}, section
}

func TestParseMessageBodiesAndBrokerMatch(t *testing.T) {
	m := &Monitor{brokers: map[string]broker.Broker{"broker.example": {ID: "acme", Name: "Acme Data"}}}

	const both = "From: privacy@broker.example\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=BB\r\n\r\n" +
		"--BB\r\nContent-Type: text/plain\r\n\r\nWe deleted your data.\r\n" +
		"--BB\r\nContent-Type: text/html\r\n\r\n<p>We <b>deleted</b> your data.</p>\r\n--BB--\r\n"
	msg, section := messageWith(both)
	email, err := m.parseMessage(msg, section)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(email.Body, "We deleted your data.") || !strings.Contains(email.HTMLBody, "<b>deleted</b>") {
		t.Fatalf("plain=%q html=%q", email.Body, email.HTMLBody)
	}
	// The sender domain is matched case-insensitively to a catalog broker.
	if email.BrokerID != "acme" || email.BrokerName != "Acme Data" || email.FromDomain != "broker.example" || email.MessageID != "<abc@broker.example>" || email.FromName != "Privacy Team" {
		t.Fatalf("envelope fields: %+v", email)
	}

	const htmlOnly = "From: privacy@broker.example\r\nMIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n<p>No records found.</p>\r\n"
	msg, section = messageWith(htmlOnly)
	email, err = m.parseMessage(msg, section)
	if err != nil || email.Body != "" || !strings.Contains(email.HTMLBody, "No records found.") {
		t.Fatalf("html-only: %+v %v", email, err)
	}
}

func TestParseMessageToleratesMissingOrBrokenBodies(t *testing.T) {
	m := &Monitor{}

	// No body section at all: the envelope is still returned.
	msg := &imap.Message{Envelope: &imap.Envelope{Subject: "only headers"}}
	email, err := m.parseMessage(msg, &imap.BodySectionName{})
	if err != nil || email.Subject != "only headers" || email.Body != "" {
		t.Fatalf("no body: %+v %v", email, err)
	}

	// Not an RFC 822 message: no body, but no failure either.
	msg, section := messageWith("\x00\x01 not a mail message")
	email, err = m.parseMessage(msg, section)
	if err != nil || email.Body != "" || email.HTMLBody != "" {
		t.Fatalf("garbage: %+v %v", email, err)
	}
}

// Brokers in Europe still send ISO-8859-1 and windows-125x mail. Without a
// charset reader the body failed to decode and arrived empty, so the reply was
// classified blind and its links were lost.
func TestParseMessageDecodesLegacyCharsets(t *testing.T) {
	m := &Monitor{brokers: map[string]broker.Broker{}}
	for _, c := range []struct{ name, charset, raw, want string }{
		{"latin1 plain", "iso-8859-1", "Wir haben Ihre Daten gel\xf6scht (K\xfcndigung).", "gelöscht (Kündigung)"},
		{"windows-1257 plain", "windows-1257", "Dati ir dz\xe7sti, \xebkums.", "dzēsti"},
		{"latin1 html", "iso-8859-1", "<p>Fran\xe7ais: supprim\xe9</p>", "supprimé"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ct := "text/plain"
			if strings.HasPrefix(c.raw, "<p>") {
				ct = "text/html"
			}
			raw := "From: privacy@broker.example\r\nMIME-Version: 1.0\r\nContent-Type: " + ct + "; charset=" + c.charset + "\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + c.raw + "\r\n"
			msg, section := messageWith(raw)
			email, err := m.parseMessage(msg, section)
			if err != nil {
				t.Fatal(err)
			}
			if got := email.Body + email.HTMLBody; !strings.Contains(got, c.want) {
				t.Errorf("body = %q, want it to contain %q", got, c.want)
			}
		})
	}
}
