package inbox

import (
	"io"
	"strings"

	"github.com/emersion/go-imap"
	// registers the readers for ISO-8859-x, windows-125x and the like; without
	// it a body in anything but UTF-8 or ASCII fails to decode and arrives empty
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
)

// maxMIMEPartBytes bounds each part read: anyone can mail this inbox.
const maxMIMEPartBytes = 10 << 20

// parseMessage builds an Email from an IMAP message: the envelope always, the
// first plain and HTML text parts when the body section was fetched. A message
// without an envelope is nil. A body that cannot be parsed gives an Email
// without a body, not an error.
func (m *Monitor) parseMessage(msg *imap.Message, section *imap.BodySectionName) (*Email, error) {
	if msg == nil || msg.Envelope == nil {
		return nil, nil
	}
	e := &Email{UID: msg.Uid, MessageID: msg.Envelope.MessageId, Subject: msg.Envelope.Subject, ReceivedAt: msg.Envelope.Date}
	if len(msg.Envelope.From) > 0 {
		from := msg.Envelope.From[0]
		e.From, e.FromName = from.Address(), from.PersonalName
		e.FromDomain = strings.ToLower(from.HostName)
	}
	if b, ok := m.brokerFor(e.From, e.FromDomain); ok {
		e.BrokerID, e.BrokerName = b.ID, b.Name
	}

	raw := msg.GetBody(section)
	if raw == nil {
		return e, nil
	}
	reader, err := mail.CreateReader(raw)
	if err != nil {
		return e, nil
	}
	for {
		part, err := reader.NextPart()
		if err != nil { // io.EOF, or a part that cannot be read: keep what we have
			break
		}
		header, ok := part.Header.(*mail.InlineHeader)
		if !ok {
			continue // an attachment
		}
		kind, _, _ := header.ContentType()
		text, _ := io.ReadAll(io.LimitReader(part.Body, maxMIMEPartBytes))
		switch {
		case strings.HasPrefix(kind, "text/plain") && e.Body == "":
			e.Body = string(text)
		case strings.HasPrefix(kind, "text/html") && e.HTMLBody == "":
			e.HTMLBody = string(text)
		}
	}
	return e, nil
}
