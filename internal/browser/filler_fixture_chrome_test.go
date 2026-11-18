package browser

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/drumandbytes/delistd/internal/config"
)

var updateFiller = flag.Bool("update-filler", false, "rewrite testdata/filler_golden.json from the current behaviour")

// The pages a filler meets: the same ten fields under different naming habits,
// plus traps (hidden fields, a select with no matching option, awkward values).
var fillerPages = map[string]string{
	"snake": `<form method="post"><input name="first_name"><input name="last_name"><input type="email" name="mail"><input type="tel" name="tel_no">
		<input name="street"><input name="city"><select name="state"><option value="">-</option><option value="CA">California</option><option>Nevada</option></select>
		<input name="zip_code"><select name="country"><option value="">-</option><option value="US">United States</option><option value="LV">Latvia</option></select>
		<input type="date" name="dob"><button type="submit">Submit</button></form>`,
	"camel": `<form method="post"><input name="firstName"><input name="lastName"><input name="email"><input name="phone"><input name="address1">
		<input id="city"><input name="state"><input name="postalCode"><input name="country"><input name="dateOfBirth"><button>Send</button></form>`,
	"placeholders": `<form method="post"><input name="a1" placeholder="First name"><input name="a2" placeholder="Last name"><input name="a3" placeholder="Your email address">
		<input name="a4" placeholder="Phone number"><input name="a5" placeholder="Street address"><input name="a6" placeholder="City"><input name="a7" placeholder="State">
		<input name="a8" placeholder="ZIP"><input name="a9" placeholder="Date of birth"><button>Go</button></form>`,
	"labels": `<form method="post"><label for="x1">Given name</label><input id="x1" name="x1"><label for="x2">Surname</label><input id="x2" name="x2">
		<label for="x3">E-mail</label><input id="x3" name="x3"><label for="x4">Mobile</label><input id="x4" name="x4"><label for="x5">Postcode</label><input id="x5" name="x5">
		<label for="x6">Town</label><input id="x6" name="x6"><button>Go</button></form>`,
	"fullname": `<form method="post"><input name="name"><input name="email"><textarea name="address"></textarea><button>Go</button></form>`,
	"autocomplete": `<form method="post"><input name="p1" autocomplete="given-name"><input name="p2" autocomplete="family-name"><input name="p3" autocomplete="email">
		<input name="p4" autocomplete="tel"><input name="p5" autocomplete="address-line1"><input name="p6" autocomplete="address-level2"><input name="p7" autocomplete="postal-code">
		<input name="p8" autocomplete="bday"><button>Go</button></form>`,
	"traps": `<form method="post"><input name="email" style="display:none"><input name="mail_visible" placeholder="email"><div style="display:none"><input name="first_name"></div>
		<input type="hidden" name="last_name"><select name="state"><option value="ZZ">Nowhere</option></select><input name="city" readonly value="locked"><button>Go</button></form>`,
	"empty": `<form method="post"><input name="comment"><button>Go</button></form>`,
}

type fillerOutcome struct {
	Success         bool
	FieldsFilled    []string
	FieldsMissing   []string
	FillErrors      []string
	SubmitAttempted bool
	Submitted       map[string]string
}

func fillerProfiles() map[string]*config.Profile {
	return map[string]*config.Profile{
		"full": {FirstName: "Jānis", LastName: "O'Brien-Smith", Email: "janis@example.com", Phone: "+371 20000000", Address: "12 \"Main\" St\\n Apt 4", City: "Rīga",
			State: "CA", ZipCode: "LV-1010", Country: "US", DateOfBirth: "1990-05-17"},
		"nevada": {FirstName: "A", LastName: "B", Email: "a@b.example", State: "Nevada", Country: "LV", Address: "line1 line2"},
		"sparse": {FirstName: "Only", Email: "only@example.com"},
	}
}

func TestFillerMatchesRecordedBehaviour(t *testing.T) {
	requireChrome(t).Close()
	got := map[string]fillerOutcome{}
	var names []string
	for p := range fillerPages {
		names = append(names, p)
	}
	sort.Strings(names)
	for pname, profile := range fillerProfiles() {
		cfg := chromeConfig(t)
		b, err := New(cfg, profile, []string{"127.0.0.1"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(b.Close)
		for _, page := range names {
			site := newOptOutSite(t, fillerPages[page])
			res, err := b.NavigateAndFill(site.URL, "acme", true)
			if err != nil && res == nil {
				t.Fatalf("%s/%s: %v", pname, page, err)
			}
			filled, missing, ferrs := append([]string{}, res.FieldsFilled...), append([]string{}, res.FieldsMissing...), append([]string{}, res.FillErrors...)
			sort.Strings(filled)
			sort.Strings(missing)
			got[pname+"/"+page] = fillerOutcome{Success: res.Success, FieldsFilled: filled, FieldsMissing: missing, FillErrors: ferrs, SubmitAttempted: res.SubmitAttempted, Submitted: site.form()}
		}
	}

	file := filepath.Join("testdata", "filler_golden.json")
	if *updateFiller {
		raw, _ := json.MarshalIndent(got, "", " ")
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]fillerOutcome
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for k, w := range want {
		if g := got[k]; !reflect.DeepEqual(g, w) {
			t.Errorf("%s differs\n got  %+v\n want %+v", k, g, w)
		}
	}
}
