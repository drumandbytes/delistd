package inbox

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/broker_replies.json: wording lifted from real broker replies, with
// the verdict each one has to get.
func TestBrokerReplies(t *testing.T) {
	raw, err := os.ReadFile("testdata/broker_replies.json")
	if err != nil {
		t.Fatal(err)
	}
	var replies []struct {
		Name, Subject, Body string
		Want                ResponseType
	}
	if err := json.Unmarshal(raw, &replies); err != nil {
		t.Fatal(err)
	}
	for _, r := range replies {
		t.Run(r.Name, func(t *testing.T) {
			got := ClassifyResponse(&Email{Subject: r.Subject, Body: r.Body})
			if got.Type != r.Want {
				t.Errorf("got %s (confidence %.2f), want %s", got.Type, got.Confidence, r.Want)
			}
			if got.Confidence == 1 && got.NeedsReview {
				t.Error("a test email should not need review")
			}
		})
	}
}
