package email

import (
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func parse(t *testing.T, raw string) *mail.Message {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("not a valid message: %v\n%s", err, raw)
	}
	return m
}

func TestFormatMessageHasDateAndPlainBody(t *testing.T) {
	now := time.Date(2026, 11, 16, 9, 30, 0, 0, time.FixedZone("EET", 2*3600))
	m := parse(t, formatMessage(Message{From: "a@x.example", To: "b@y.example", Subject: "Erasure request", Body: "Line one\nLine two"}, "<id@x.example>", now))
	if got, err := m.Header.Date(); err != nil || !got.Equal(now) {
		t.Errorf("Date = %v, %v; want %v", got, err, now)
	}
	if m.Header.Get("Content-Transfer-Encoding") != "7bit" || m.Header.Get("Subject") != "Erasure request" {
		t.Errorf("headers = %v", m.Header)
	}
	body, _ := io.ReadAll(m.Body)
	if string(body) != "Line one\nLine two" {
		t.Errorf("body = %q", body)
	}
}

// Accented names are normal in the EU: neither the subject nor the body may
// depend on a server accepting raw 8-bit mail.
func TestFormatMessageEncodesNonASCII(t *testing.T) {
	m := parse(t, formatMessage(Message{From: "a@x.example", To: "b@y.example", Subject: "Dzēšanas pieprasījums: Jānis Bērziņš", Body: "Lūdzu dzēsiet manus datus.\nJānis"}, "<id@x.example>", time.Now()))
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != "Dzēšanas pieprasījums: Jānis Bērziņš" {
		t.Errorf("subject = %q, %v (raw %q)", subject, err, m.Header.Get("Subject"))
	}
	if strings.ContainsAny(m.Header.Get("Subject"), "ēšāīņ") {
		t.Errorf("raw subject still has 8-bit characters: %q", m.Header.Get("Subject"))
	}
	if m.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
		t.Fatalf("encoding = %q", m.Header.Get("Content-Transfer-Encoding"))
	}
	body, _ := io.ReadAll(quotedprintable.NewReader(m.Body))
	if !strings.Contains(string(body), "Lūdzu dzēsiet manus datus.") {
		t.Errorf("decoded body = %q", body)
	}
}

func TestPlainEnough(t *testing.T) {
	for _, c := range []struct {
		text string
		want bool
	}{
		{"hello\nworld", true},
		{"", true},
		{"naïve", false},
		{"a\x00b", false},
		{strings.Repeat("x", 998), true},
		{strings.Repeat("x", 999), false},
		{strings.Repeat("x", 998) + "\n" + strings.Repeat("y", 998), true},
	} {
		if got := plainEnough(c.text); got != c.want {
			t.Errorf("plainEnough(%.20q...) = %v, want %v", c.text, got, c.want)
		}
	}
}
