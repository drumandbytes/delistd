package inbox

import (
	"slices"
	"testing"
)

func TestNormalizeURLRefusesPrivateTargets(t *testing.T) {
	for raw, ok := range map[string]bool{
		"http://169.254.169.254/latest/meta-data/":        false, // cloud metadata
		"http://localhost:8080/opt-out":                   false,
		"http://LOCALHOST/opt-out":                        false,
		"http://127.0.0.1/opt-out":                        false,
		"http://[::1]/opt-out":                            false,
		"http://10.0.0.5/opt-out":                         false,
		"http://192.168.1.1/opt-out":                      false,
		"http://172.16.0.1/opt-out":                       false,
		"http://0.0.0.0/opt-out":                          false,
		"http://169.254.1.1/opt-out":                      false,
		"ftp://example.com/opt-out":                       false,
		"https:///nohost":                                 false,
		"http://example.com/opt-out":                      true,
		"https://www.somebroker.com/privacy-request?id=1": true,
		"http://8.8.8.8/opt-out":                          true,
		"https://example.com/opt-out).":                   true, // trailing punctuation is trimmed
	} {
		if got := normalizeURL(raw) != ""; got != ok {
			t.Errorf("normalizeURL(%q) kept = %v, want %v", raw, got, ok)
		}
	}
}

func TestPrivateHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "127.0.0.2": true, "0.0.0.0": true, "::1": true, "10.1.2.3": true,
		"172.16.5.5": true, "172.31.255.255": true, "192.168.0.1": true, "169.254.169.254": true,
		"224.0.0.1":   true,
		"example.com": false, "8.8.8.8": false, "172.32.0.1": false,
		"internal.corp": false, // a name, and names are not resolved
	} {
		if got := privateHost(host); got != want {
			t.Errorf("privateHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestLinksInHTML(t *testing.T) {
	doc := `<p>Confirm <a class="btn" href="https://acme.example/confirm?t=1&amp;u=2">here</a>
	or visit https://acme.example/portal.</p><a name="x">no href</a><img src="https://img.example/x.png">`
	want := []string{"https://acme.example/confirm?t=1&u=2", "https://acme.example/portal."}
	if got := linksInHTML(doc); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestIsTracker(t *testing.T) {
	for url, want := range map[string]bool{
		"https://broker.example/open.gif":        true,
		"https://broker.example/pixel/a.png":     true,
		"https://broker.example/logo.png":        false,
		"https://broker.example/privacy/opt-out": false,
	} {
		if got := isTracker(url); got != want {
			t.Errorf("isTracker(%q) = %v", url, got)
		}
	}
}
