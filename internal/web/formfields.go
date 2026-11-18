package web

import (
	"html/template"
	"strings"

	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/country"
)

// The forms are described as data and drawn by partials/field.html, so a
// field is declared once (what it is called, how it validates, what it hints)
// and every form that shows it looks the same.

type selectOption struct {
	Value, Label string
	Selected     bool
}

type formField struct {
	Name, Label, Type, Value, Placeholder, Autocomplete string
	Required                                            bool
	Hint, HintClass                                     string
	Error                                               string
	Attrs                                               template.HTMLAttr // other attributes, as written
	Span                                                string            // class of the field's wrapper
	Rows                                                int               // > 0 makes it a textarea
	Options                                             []selectOption    // non-empty makes it a select
}

// formRow is fields side by side in a grid; an empty Class draws them one
// below the other.
type formRow struct {
	Class  string
	Fields []formField
}

type introLine struct{ Class, Text string }

// formBlock is a group of rows, optionally set off by a rule and an intro.
type formBlock struct {
	Class     string // the block's wrapper; empty for none
	Intro     []introLine
	BodyClass string // wrapper around the rows; empty for none
	Rows      []formRow
}

const (
	hintClass  = "mt-1 text-sm text-muted"
	hintSmall  = "mt-1 text-xs text-muted"
	ruledBlock = "pt-6 border-t"
)

// lines is how a textarea shows a list: one entry per line, each ended by a newline.
func lines(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, "\n") + "\n"
}

// profileForm lays out the profile fields for a form, filled from p and
// flagged with errs (field name to message). errs may be nil.
func profileForm(p config.Profile, errs map[string]string) []formBlock {
	field := func(name, label, typ, value, placeholder, autocomplete string) formField {
		return formField{Name: name, Label: label, Type: typ, Value: value, Placeholder: placeholder, Autocomplete: autocomplete, Error: errs[name], HintClass: hintClass}
	}
	required := func(f formField) formField { f.Required = true; return f }
	list := func(name, label, placeholder string, items []string, hint string) formField {
		f := field(name, label, "", lines(items), placeholder, "")
		f.Rows, f.Hint, f.HintClass = 2, hint, hintSmall
		return f
	}

	current, _ := country.Code(p.Country)
	options := []selectOption{{"", "-", false}}
	for _, c := range country.All() {
		options = append(options, selectOption{c.Code, c.Name, c.Code == current})
	}
	countryField := field("country", "Country", "", "", "", "country")
	countryField.Options = options

	middle := field("middle_name", "Middle Name", "text", p.MiddleName, "Optional", "additional-name")
	middle.Hint = "Some brokers (especially background-check sites) match on full legal name"
	email := required(field("email", "Email Address", "email", p.Email, "jane@example.com", "email"))
	email.Hint = "This email will receive responses from data brokers, and identifies this profile in removal requests"
	city := field("city", "City", "text", p.City, "New York", "address-level2")
	city.Span = "col-span-2"
	zip := field("zip_code", "ZIP", "text", p.ZipCode, "10001", "postal-code")
	zip.Attrs = `maxlength="10" pattern="[A-Za-z0-9][A-Za-z0-9 \-]{1,8}[A-Za-z0-9]" title="Letters, digits, spaces or hyphens (3 to 10 characters)"`
	phone := field("phone", "Phone", "tel", p.Phone, "+1 (555) 123-4567", "tel")
	phone.Attrs = `maxlength="30" pattern="[0-9+()\-. /]{6,30}" title="6 to 15 digits; spaces, + ( ) - . are fine"`
	dob := field("date_of_birth", "Date of Birth", "date", p.DateOfBirth, "", "bday")
	dob.Attrs = `min="1900-01-01"`
	prev := list("previous_addresses", "Previous Addresses", "456 Old Street, San Francisco, CA", p.PreviousAddresses, "From roughly the last 5-7 years. A partial address still helps.")

	return []formBlock{
		{Rows: []formRow{
			{Class: "grid grid-cols-2 gap-4", Fields: []formField{
				required(field("first_name", "First Name", "text", p.FirstName, "Jane", "given-name")),
				required(field("last_name", "Last Name", "text", p.LastName, "Doe", "family-name")),
			}},
			{Fields: []formField{middle}},
			{Fields: []formField{email}},
		}},
		{Class: ruledBlock, Intro: []introLine{{"text-sm text-muted mb-4", "Optional: Helps brokers find your records"}}, BodyClass: "space-y-4", Rows: []formRow{
			{Fields: []formField{field("address", "Street Address", "text", p.Address, "123 Main St", "street-address")}},
			{Class: "grid grid-cols-2 sm:grid-cols-4 gap-3", Fields: []formField{city, field("state", "State", "text", p.State, "NY", "address-level1"), zip}},
			{Class: "grid grid-cols-2 gap-4", Fields: []formField{countryField, phone}},
			{Fields: []formField{dob}},
		}},
		{Class: ruledBlock, Intro: []introLine{
			{"text-sm text-muted mb-1", "Optional: other names, addresses and contacts"},
			{"text-xs text-muted mb-4", "Brokers often hold records under an old address, a maiden name or a spelling without diacritics. One per line."},
		}, BodyClass: "space-y-4", Rows: []formRow{
			{Fields: []formField{list("name_variants", "Other Names", "Jane D.\nJane Smith", p.NameVariants, "")}},
			{Fields: []formField{list("additional_emails", "Other Email Addresses", "old.address@example.com", p.AdditionalEmails, "")}},
			{Fields: []formField{list("additional_phones", "Other Phone Numbers", "+1 (555) 987-6543", p.AdditionalPhones, "")}},
			{Fields: []formField{prev}},
		}},
	}
}

// profileFormFor is profileForm for a template, which may hand over a nil or
// missing Errors and a profile that is not yet set.
func profileFormFor(profile any, errs any) []formBlock {
	p, _ := profile.(config.Profile)
	e, _ := errs.(map[string]string)
	return profileForm(p, e)
}
