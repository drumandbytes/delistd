package email

import (
	"fmt"
	"net/mail"
	"strings"
)

// Message is one email to send.
type Message struct {
	To      string
	From    string
	Subject string
	Body    string
}

// Result is the outcome of a send. Success is false when Error is set.
type Result struct {
	Success   bool
	MessageID string
	Error     error
}

// ValidateEmail rejects characters that could inject headers or extra
// recipients, then anything net/mail won't parse as an address.
func ValidateEmail(address string) error {
	if strings.ContainsAny(address, "\r\n,;") {
		return fmt.Errorf("email contains invalid characters")
	}
	if _, err := mail.ParseAddress(address); err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	return nil
}

// validateMessage checks both addresses of msg.
func validateMessage(msg Message) error {
	for _, check := range []struct{ role, address string }{{"sender", msg.From}, {"recipient", msg.To}} {
		if err := ValidateEmail(check.address); err != nil {
			return fmt.Errorf("invalid %s: %w", check.role, err)
		}
	}
	return nil
}
