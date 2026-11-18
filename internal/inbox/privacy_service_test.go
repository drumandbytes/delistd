package inbox

import (
	"testing"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
)

// Wording of replies relayed by privacy services. Written for the test, not
// lifted from a real reply: check a new case against a real one before relying on it.
func TestPrivacyServiceReplies(t *testing.T) {
	cases := []struct {
		name, body string
		want       ResponseType
	}{
		{"opt-out processed", "We have now processed your opt-out request.", ResponseSuccess},
		{"marked for removal", "Your record has been marked for removal.", ResponseSuccess},
		{"data permanently deleted", "Your personal data has been permanently deleted from our systems.", ResponseSuccess},
		{"exercise request completed", "Your request to exercise your right to erasure has been completed.", ResponseSuccess},
		{"typed request received", "We received your Deletion request and will be in touch.", ResponsePending},
		{"another email follows", "You will receive another email when this is done.", ResponsePending},
		{"once processed", "Once we have processed it, we will write again.", ResponsePending},
		// a promise is not a confirmation: it must never score as success
		{"promise of deletion", "Your data will be permanently deleted within 45 days.", ResponseUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyResponse(&Email{Subject: "Your privacy request", Body: c.body}); got.Type != c.want {
				t.Errorf("got %s, want %s", got.Type, c.want)
			}
		})
	}
}

// A broker on a free mailbox owns its address, not the whole domain: other
// senders on the same provider are not that broker.
func TestFreeMailBrokerMatchesByAddress(t *testing.T) {
	m := NewMonitor(config.InboxConfig{}, []broker.Broker{
		{ID: "solo", Name: "Solo Lookups", Email: "Solo.Privacy@gmail.com"},
		{ID: "acme", Name: "Acme", Email: "privacy@acme.example"},
	})
	for _, c := range []struct {
		from, domain, want string
	}{
		{"solo.privacy@gmail.com", "gmail.com", "solo"},
		{"SOLO.PRIVACY@gmail.com", "gmail.com", "solo"},
		{"someone.else@gmail.com", "gmail.com", ""},
		{"privacy@acme.example", "acme.example", "acme"},
		{"x@unknown.example", "unknown.example", ""},
		{"", "", ""},
	} {
		got, _ := m.brokerFor(c.from, c.domain)
		if got.ID != c.want {
			t.Errorf("%s: matched %q, want %q", c.from, got.ID, c.want)
		}
	}
}
