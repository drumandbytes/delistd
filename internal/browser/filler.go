package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"

	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/country"
)

// isContextErr tells a dead browser (the context was cancelled or timed out)
// from chromedp's ordinary "no such element", so a missing field is not
// mistaken for a failure and a failure is not reported as success.
func isContextErr(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// FormFiller finds the fields of an opt-out form and types the profile into them.
type FormFiller struct {
	profile *config.Profile
}

// FillResult is what a fill did.
type FillResult struct {
	FilledFields  []string
	MissingFields []string // profile fields with no value to enter
	Errors        []string // real failures such as a timeout, not just "no such field on this page"
}

// FieldMapping is one profile field and how to find its input on a page.
type FieldMapping struct {
	FieldType    string
	ProfileValue string
	Selectors    []string // CSS selectors, most specific first
	Patterns     []string // fragments looked for in an input's name, id, placeholder and label
}

func NewFormFiller(profile *config.Profile) *FormFiller { return &FormFiller{profile: profile} }

// fieldSpec is a field's static part. Selectors are separated by " | " and
// patterns by spaces. The order of the selectors is the order they are tried.
type fieldSpec struct {
	name      string
	value     func(*config.Profile) string
	selectors string
	patterns  string
}

var fieldSpecs = []fieldSpec{
	{"email", func(p *config.Profile) string { return p.Email },
		`input[type='email'] | input[name='email'] | input[id='email'] | input[name*='email'] | input[id*='email'] | input[placeholder*='email' i] | input[autocomplete='email']`,
		"email e-mail e_mail"},
	{"firstName", func(p *config.Profile) string { return p.FirstName },
		`input[name='firstName'] | input[name='first_name'] | input[name='fname'] | input[id='firstName'] | input[id='first_name'] | input[name*='first'] | input[id*='first'] | input[placeholder*='first name' i] | input[autocomplete='given-name']`,
		"first fname given"},
	{"lastName", func(p *config.Profile) string { return p.LastName },
		`input[name='lastName'] | input[name='last_name'] | input[name='lname'] | input[id='lastName'] | input[id='last_name'] | input[name*='last'] | input[id*='last'] | input[placeholder*='last name' i] | input[autocomplete='family-name']`,
		"last lname family surname"},
	{"fullName", func(p *config.Profile) string { return p.FullName() },
		`input[name='name'] | input[name='fullName'] | input[name='full_name'] | input[id='name'] | input[id='fullName'] | input[placeholder*='full name' i] | input[placeholder*='your name' i] | input[autocomplete='name']`,
		"fullname full_name your_name"},
	{"phone", func(p *config.Profile) string { return p.Phone },
		`input[type='tel'] | input[name='phone'] | input[name='telephone'] | input[name='mobile'] | input[id='phone'] | input[name*='phone'] | input[id*='phone'] | input[placeholder*='phone' i] | input[autocomplete='tel']`,
		"phone tel mobile cell"},
	{"address", func(p *config.Profile) string { return p.Address },
		`input[name='address'] | input[name='street'] | input[name='address1'] | input[name='streetAddress'] | input[id='address'] | input[name*='address'] | input[name*='street'] | input[placeholder*='address' i] | input[placeholder*='street' i] | input[autocomplete='street-address'] | input[autocomplete='address-line1']`,
		"address street addr"},
	{"city", func(p *config.Profile) string { return p.City },
		`input[name='city'] | input[id='city'] | input[name*='city'] | input[placeholder*='city' i] | input[autocomplete='address-level2']`,
		"city town locality"},
	{"state", func(p *config.Profile) string { return p.State },
		`input[name='state'] | input[id='state'] | input[name*='state'] | input[placeholder*='state' i] | select[name='state'] | select[id='state'] | input[autocomplete='address-level1']`,
		"state province region"},
	{"zipCode", func(p *config.Profile) string { return p.ZipCode },
		`input[name='zip'] | input[name='zipCode'] | input[name='zip_code'] | input[name='postalCode'] | input[name='postal_code'] | input[id='zip'] | input[name*='zip'] | input[name*='postal'] | input[placeholder*='zip' i] | input[placeholder*='postal' i] | input[autocomplete='postal-code']`,
		"zip postal postcode"},
	{"country", func(p *config.Profile) string { return country.Display(p.Country) },
		`input[name='country'] | input[id='country'] | select[name='country'] | select[id='country'] | input[autocomplete='country-name']`,
		"country nation"},
	{"dateOfBirth", func(p *config.Profile) string { return p.DateOfBirth },
		`input[name='dob'] | input[name='dateOfBirth'] | input[name='date_of_birth'] | input[name='birthdate'] | input[name='birth_date'] | input[type='date'] | input[id='dob'] | input[placeholder*='birth' i] | input[autocomplete='bday']`,
		"dob birth bday"},
}

func (f *FormFiller) getFieldMappings() []FieldMapping {
	out := make([]FieldMapping, len(fieldSpecs))
	for i, s := range fieldSpecs {
		out[i] = FieldMapping{
			FieldType:    s.name,
			ProfileValue: s.value(f.profile),
			Selectors:    strings.Split(s.selectors, " | "),
			Patterns:     strings.Fields(s.patterns),
		}
	}
	return out
}

// Fill enters every profile field it can find on the page.
func (f *FormFiller) Fill(ctx context.Context) *FillResult {
	result := &FillResult{}
	for _, m := range f.getFieldMappings() {
		if m.ProfileValue == "" {
			result.MissingFields = append(result.MissingFields, m.FieldType)
			continue
		}
		filled, err := f.fillField(ctx, m)
		switch {
		case err != nil:
			// a real failure, not an absent field: kept apart so a fill is
			// not reported as a success just because some other field went in
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", m.FieldType, err))
		case filled:
			result.FilledFields = append(result.FilledFields, m.FieldType)
		}
	}
	return result
}

type candidate struct {
	Selector string `json:"selector"`
	Tag      string `json:"tag"`
	Type     string `json:"type"`
}

// findCandidates asks the page, in one round trip, which of the field's
// selectors point at a visible element, and which input its name, id,
// placeholder or label suggest as a last resort. It looks at the page as it is
// now, so a field a previous fill revealed (a state list after the country) is
// found.
const findCandidatesJS = `(function(spec) {
	function visible(sel) {
		try {
			var el = document.querySelector(sel);
			return el !== null && el.offsetParent !== null ? {tag: el.tagName.toLowerCase(), type: (el.type || '').toLowerCase()} : null;
		} catch (e) { return null; }
	}
	var found = [];
	spec.selectors.forEach(function(sel) {
		var v = visible(sel);
		if (v) found.push({selector: sel, tag: v.tag, type: v.type});
	});
	var inputs = document.querySelectorAll('input, select, textarea');
	for (var i = 0; i < inputs.length; i++) {
		var el = inputs[i];
		var text = [el.name, el.id, el.placeholder].map(function(s) { return (s || '').toLowerCase(); });
		if (el.id) {
			var label = document.querySelector('label[for="' + el.id + '"]');
			if (label) text.push(label.textContent.toLowerCase());
		}
		var hit = spec.patterns.some(function(p) { return text.some(function(t) { return t.includes(p); }); });
		if (!hit || el.offsetParent === null) continue;
		var sel = el.tagName.toLowerCase() + (el.id ? '#' + el.id : (el.name ? '[name="' + el.name + '"]' : ''));
		var v = visible(sel);
		if (v) found.push({selector: sel, tag: v.tag, type: v.type});
		break;
	}
	return found;
})(`

func (f *FormFiller) findCandidates(ctx context.Context, m FieldMapping) ([]candidate, error) {
	spec, _ := json.Marshal(map[string]any{"selectors": m.Selectors, "patterns": m.Patterns})
	found, err := chromedp.Run(ctx, chromedp.Evaluate[[]candidate](findCandidatesJS+string(spec)+`)`))
	if err != nil {
		if isContextErr(err) {
			return nil, err
		}
		return nil, nil // the page would not run it: treat as nothing found
	}
	return found, nil
}

// fillField tries each candidate in order until one takes the value. The
// error is non-nil only for a real failure; a field that is not on the page is
// (false, nil).
func (f *FormFiller) fillField(ctx context.Context, m FieldMapping) (bool, error) {
	candidates, err := f.findCandidates(ctx, m)
	if err != nil {
		return false, err
	}
	tried := map[string]bool{}
	for _, c := range candidates {
		if tried[c.Selector] {
			continue
		}
		tried[c.Selector] = true
		var ok bool
		switch {
		case c.Tag == "select":
			ok, err = f.chooseOption(ctx, c.Selector, m.ProfileValue)
		case c.Type == "date":
			ok, err = f.setValue(ctx, c.Selector, m.ProfileValue)
		default:
			ok, err = f.typeInto(ctx, c.Selector, singleLine(c.Tag, m.ProfileValue))
		}
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// singleLine keeps a newline out of a one-line input: typing Enter there
// submits the form half filled. A textarea takes the value as it is.
func singleLine(tag, value string) string {
	if tag == "textarea" {
		return value
	}
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(value)
}

// typeInto clears the input and types the value.
func (f *FormFiller) typeInto(ctx context.Context, selector, value string) (bool, error) {
	if err := chromedp.Do(ctx, chromedp.Clear(selector), chromedp.SendKeys(selector, value)); err != nil {
		if isContextErr(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// evalOnElement runs body, a JS function body over (el, value), against the
// first element matching selector, and reports whether it returned true. A
// failing script is (false, nil) so the caller can fall through to the next
// way of filling the field; only a dead context is an error.
func (f *FormFiller) evalOnElement(ctx context.Context, selector, value, body string) (bool, error) {
	sel, _ := json.Marshal(selector)
	want, _ := json.Marshal(value)
	js := fmt.Sprintf(`(function(el, value) { if (!el) return false; %s })(document.querySelector(%s), %s)`, body, sel, want)
	ok, err := chromedp.Run(ctx, chromedp.Evaluate[bool](js))
	if err != nil {
		if isContextErr(err) {
			return false, err
		}
		return false, nil
	}
	return ok, nil
}

// chooseOption picks the <option> whose value or text is value and fires
// 'change'. Typing into a <select> can report success while selecting
// nothing, so this sets .value against a real option instead. No matching
// option is (false, nil).
func (f *FormFiller) chooseOption(ctx context.Context, selector, value string) (bool, error) {
	return f.evalOnElement(ctx, selector, value, `
		var opt = Array.from(el.options).find(function(o) { return o.value === value || o.textContent.trim() === value; });
		if (!opt) return false;
		el.value = opt.value;
		el.dispatchEvent(new Event('change', {bubbles: true}));
		return el.value === opt.value;`)
}

// setValue sets the value directly and fires the events a keystroke would. A
// date input has to be filled this way: typed keys are read in the browser's
// own date order (month first in en-US, day first elsewhere), so an ISO date
// typed into it ends up as a different date from machine to machine.
func (f *FormFiller) setValue(ctx context.Context, selector, value string) (bool, error) {
	return f.evalOnElement(ctx, selector, value, `
		el.value = value;
		el.dispatchEvent(new Event('input', {bubbles: true}));
		el.dispatchEvent(new Event('change', {bubbles: true}));
		return el.value === value;`)
}
