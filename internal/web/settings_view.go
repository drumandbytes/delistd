package web

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/drumandbytes/delistd/internal/config"
)

// optField describes one control of the request-options form, drawn by
// partials/opt.html.
type optField struct {
	Label, ID, Name, Kind, Type, Value, Attrs, Placeholder, Class string
	Hint                                                          template.HTML
	Options                                                       []selectOption
}

const customFields = "{{.FullName}} {{.FirstName}} {{.LastName}} {{.Email}} {{.Address}} {{.City}} {{.State}} {{.ZipCode}} {{.Country}} {{.Phone}} {{.DateOfBirth}} {{.OtherNames}} {{.OtherEmails}} {{.OtherAddresses}} {{.OtherPhones}} {{.BrokerName}} {{.BrokerEmail}} {{.BrokerWebsite}} {{.BrokerOptOut}} {{.Date}} {{.Year}} {{.Month}}"

// optionFields lays out the request options in the order the form shows them.
// The checkbox for sending by hand and the region boxes sit between these and
// are drawn by the page itself.
func optionFields(cfg *config.Config) (before []optField, after []optField) {
	o := cfg.Options
	templates := make([]selectOption, 0, len(requestTemplates)+1)
	for _, t := range requestTemplates {
		templates = append(templates, selectOption{t, t, t == o.Template})
	}
	templates = append(templates, selectOption{"custom", "custom (your own text, below)", o.Template == "custom"})

	list := []selectOption{
		{"full", "full (all brokers)", o.BrokerList != "verified"},
		{"verified", "verified (hand-checked subset)", o.BrokerList == "verified"},
	}
	before = []optField{
		{Label: "Default Template", ID: "opt-template", Name: "template", Kind: "select", Options: templates},
		{Label: "Custom template text", ID: "opt-custom", Name: "custom_template", Kind: "textarea", Value: o.CustomTemplate, Class: "font-mono text-xs",
			Placeholder: "Subject: Data erasure request\n\nTo {{.BrokerName}},\n...\n{{.FullName}}",
			Hint: template.HTML(`Used when the template above is "custom". An optional first line <code>Subject: ...</code> sets the subject. Fields: ` +
				template.HTMLEscapeString(customFields))},
		{Label: "Rate Limit (ms between emails)", ID: "opt-rate", Name: "rate_limit_ms", Kind: "input", Type: "number", Attrs: `min="0"`, Value: strconv.Itoa(o.RateLimitMs)},
	}
	after = []optField{
		{Label: "Daily send limit", ID: "opt-daily", Name: "daily_send_limit", Kind: "input", Type: "number", Attrs: `min="1"`, Value: strconv.Itoa(o.DailySendLimit)},
		{Label: "Broker list", ID: "opt-list", Name: "broker_list", Kind: "select", Options: list},
		{Label: "Excluded categories (comma-separated, e.g. requires-id)", ID: "opt-cats", Name: "excluded_categories", Kind: "input", Type: "text", Value: strings.Join(o.ExcludedCategories, ", ")},
	}
	return before, after
}
