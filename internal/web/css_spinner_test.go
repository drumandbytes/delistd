package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A spinner nested in a plain wrapper span (the bulk-send and retry buttons) is
// an inline box, where width and height do nothing and only a border fragment
// shows (#313). It has to size itself.
func TestSpinnerSizesItself(t *testing.T) {
	css, err := os.ReadFile("static/css/app.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`(?m)^\.spinner \{([^}]*)\}`).FindSubmatch(css)
	if rule == nil {
		t.Fatal(".spinner rule not found in app.css")
	}
	if !strings.Contains(string(rule[1]), "display: inline-block") {
		t.Errorf(".spinner needs display: inline-block, got: %s", rule[1])
	}
}
