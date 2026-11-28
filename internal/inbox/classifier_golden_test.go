package inbox

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"sync"
	"testing"
)

// testdata/classifier_golden.json.gz holds ~7,000 emails built from every
// phrase the classifier knows (alone, combined, in subjects, with and without
// links, from bounce senders) and the verdicts the classifier gave them before
// it was rewritten. Where two types tied on points the old code picked one at
// random, so such a case lists every type it was seen to pick and only the
// type is compared.
//
// A tie is now settled the same way every time and always sent to review, so
// for those cases the test only requires one of the tied types plus the review flag.
type goldenCase struct {
	In  struct{ From, FromName, FromDomain, Subject, Body, HTMLBody string }
	Out struct {
		Types                                []string
		Confidence                           float64
		NeedsReview                          bool
		Reason, FormURL, ConfirmURL, Bounced string
		SubjTypes                            []string
		SubjConf                             float64
		SubjReview                           bool
	}
}

func TestClassifierMatchesGolden(t *testing.T) {
	f, err := os.Open("testdata/classifier_golden.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	// the cases are independent; spread them over the CPUs, which matters
	// under -race where this is the slowest test in the package
	workers := runtime.GOMAXPROCS(0)
	failures := make([][]string, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := w; i < len(cases); i += workers {
				failures[w] = append(failures[w], checkGoldenCase(i, cases[i])...)
			}
		}()
	}
	wg.Wait()
	bad := 0
	for _, fs := range failures {
		for _, f := range fs {
			if bad++; bad <= 15 {
				t.Error(f)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d cases differ from the golden file (of %d)", bad, len(cases))
	}
}

func checkGoldenCase(i int, c goldenCase) (problems []string) {
	in := c.In
	fail := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf("case %d (%q / %q): "+format, append([]any{i, in.Subject, in.Body}, args...)...))
	}
	got := ClassifyResponse(&Email{From: in.From, FromName: in.FromName, FromDomain: in.FromDomain, Subject: in.Subject, Body: in.Body, HTMLBody: in.HTMLBody})
	subject, content := emailText(&Email{Subject: in.Subject, Body: in.Body, HTMLBody: in.HTMLBody})
	_, top, runnerUp := scoreReply(subject, content, got.URLs).top()
	viaScoring := slices.Contains(scored[:], got.Type) && got.Confidence < 1 // not a test email
	switch o := c.Out; {
	case viaScoring && top > 0 && top == runnerUp:
		if !got.NeedsReview {
			fail("tie on %d points not sent to review (type %s)", top, got.Type)
		}
	case !slices.Contains(o.Types, string(got.Type)):
		fail("type %s, golden %v", got.Type, o.Types)
	case len(o.Types) == 1 && (!near(got.Confidence, o.Confidence) || got.NeedsReview != o.NeedsReview || got.Reason != o.Reason ||
		got.FormURL != o.FormURL || got.ConfirmURL != o.ConfirmURL || got.BouncedRecipient != o.Bounced):
		fail("got conf %v review %v reason %q form %q confirm %q bounced %q; golden conf %v review %v reason %q form %q confirm %q bounced %q",
			got.Confidence, got.NeedsReview, got.Reason, got.FormURL, got.ConfirmURL, got.BouncedRecipient,
			o.Confidence, o.NeedsReview, o.Reason, o.FormURL, o.ConfirmURL, o.Bounced)
	}
	ty, cf, rv := ClassifyBySubjectOnly(in.Subject)
	if o := c.Out; !slices.Contains(o.SubjTypes, string(ty)) || (len(o.SubjTypes) == 1 && (!near(cf, o.SubjConf) || rv != o.SubjReview)) {
		fail("subject-only %s %v %v, golden %v %v %v", ty, cf, rv, o.SubjTypes, o.SubjConf, o.SubjReview)
	}
	return problems
}

// near: fused multiply-add on arm64 makes the last bit of a float differ from amd64.
func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
