package inbox

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// testdata/parser_golden.json.gz: ~4,200 emails (links in text and HTML,
// private hosts, tracking pixels, bounce notices) and what the link and
// bounce-recipient code made of them before it was rewritten.
func TestParserMatchesGolden(t *testing.T) {
	f, err := os.Open("testdata/parser_golden.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		In struct {
			From, FromName, Subject, Body, HTMLBody string
			Domains                                 []string
		}
		Out struct {
			Form, Confirm, Unsub, All, PrimaryForm, PrimaryConfirm []string
			Bounced                                                string
		}
	}
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	bad := 0
	for i, c := range cases {
		e := &Email{From: c.In.From, FromName: c.In.FromName, Subject: c.In.Subject, Body: c.In.Body, HTMLBody: c.In.HTMLBody}
		u := ParseEmailURLs(e)
		var pf, pc []string
		for _, d := range c.In.Domains {
			pf = append(pf, GetPrimaryFormURL(u, d))
			pc = append(pc, GetPrimaryConfirmationURL(u, d))
		}
		same := reflect.DeepEqual(append([]string(nil), u.FormURLs...), append([]string(nil), c.Out.Form...)) &&
			reflect.DeepEqual(append([]string(nil), u.ConfirmationURLs...), append([]string(nil), c.Out.Confirm...)) &&
			reflect.DeepEqual(append([]string(nil), u.UnsubscribeURLs...), append([]string(nil), c.Out.Unsub...)) &&
			reflect.DeepEqual(append([]string(nil), u.AllURLs...), append([]string(nil), c.Out.All...)) &&
			reflect.DeepEqual(pf, c.Out.PrimaryForm) && reflect.DeepEqual(pc, c.Out.PrimaryConfirm) &&
			ExtractBouncedRecipient(e) == c.Out.Bounced
		if !same {
			if bad++; bad <= 10 {
				t.Errorf("case %d differs\n body %q\n html %q\n got  %+v primary %q %q bounced %q\n want %+v primary %q %q bounced %q",
					i, c.In.Body, c.In.HTMLBody, u, pf, pc, ExtractBouncedRecipient(e), c.Out, c.Out.PrimaryForm, c.Out.PrimaryConfirm, c.Out.Bounced)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d cases differ from the golden file", bad, len(cases))
	}
}
