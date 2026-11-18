package template

import (
	"fmt"
	"mime"
	"strings"
)

// FormatEML wraps a rendered email as an RFC-822 message that a mail client
// can open directly (double-click a .eml, or File > Import). The From line is
// the data subject's own address; there is no Date header so the client stamps
// it when the user actually sends.
func FormatEML(from, to string, email *Email) []byte {
	var b strings.Builder
	if from != "" {
		fmt.Fprintf(&b, "From: %s\r\n", from)
	}
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", email.Subject)) // RFC 2047: some clients mangle a raw UTF-8 subject
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(email.Body, "\n", "\r\n"))
	if !strings.HasSuffix(email.Body, "\n") {
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}
