// Package template renders the removal-request emails.
package template

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/country"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

// CustomName is the template name backed by Options.CustomTemplate.
const CustomName = "custom"

// subjects maps a built-in template to its subject line; anything else gets defaultSubject.
var subjects = map[string]string{
	"gdpr": "GDPR Data Erasure Request - Article 17 Right to Erasure",
	"ccpa": "CCPA Data Deletion Request - Right to Delete Personal Information",
}

const defaultSubject = "Personal Data Removal Request"

// EmailData is everything a template can reference. The Other* fields join a
// profile's alternate identities and are empty when there are none.
type EmailData struct {
	// the person
	FirstName   string
	LastName    string
	FullName    string
	DateOfBirth string

	// contact details
	Email          string
	OtherEmails    string // comma-separated
	Phone          string
	OtherPhones    string // comma-separated
	OtherNames     string // comma-separated name variants
	Address        string
	City           string
	State          string
	ZipCode        string
	Country        string
	OtherAddresses string // semicolon-separated previous addresses

	// the broker
	BrokerName    string
	BrokerEmail   string
	BrokerWebsite string
	BrokerOptOut  string

	// when and what
	Date     string
	Year     int
	Month    string
	Template string
}

// Email is a rendered message.
type Email struct {
	Subject string
	Body    string
}

// Engine renders the built-in templates plus an optional custom one.
type Engine struct {
	mu            sync.RWMutex
	byName        map[string]*template.Template
	customSubject string // "" = use the default subject
}

// NewEngine parses every embedded template.
func NewEngine() (*Engine, error) {
	entries, err := embeddedTemplates.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("read embedded templates: %w", err)
	}
	e := &Engine{byName: make(map[string]*template.Template, len(entries))}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), path.Ext(entry.Name()))
		src, err := embeddedTemplates.ReadFile(path.Join("templates", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read embedded template %s: %w", name, err)
		}
		if e.byName[name], err = template.New(name).Parse(string(src)); err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
	}
	return e, nil
}

// ParseCustom splits an optional leading "Subject: ..." line off body and
// parses the rest as a template.
func ParseCustom(body string) (*template.Template, string, error) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	var subject string
	if first, rest, _ := strings.Cut(body, "\n"); strings.HasPrefix(strings.ToLower(first), "subject:") {
		subject = strings.TrimSpace(first[len("subject:"):])
		body = strings.TrimLeft(rest, "\n")
	}
	if strings.TrimSpace(body) == "" {
		return nil, "", fmt.Errorf("template body is empty")
	}
	tmpl, err := template.New(CustomName).Option("missingkey=error").Parse(body)
	if err != nil {
		return nil, "", fmt.Errorf("invalid template: %w", err)
	}
	return tmpl, subject, nil
}

// CheckCustom reports whether body parses and renders, so a bad field name
// fails here and not at send time.
func CheckCustom(body string) error {
	tmpl, _, err := ParseCustom(body)
	if err != nil {
		return err
	}
	return tmpl.Execute(io.Discard, EmailData{})
}

// SetCustom installs the custom template, or removes it when body is blank.
func (e *Engine) SetCustom(body string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if strings.TrimSpace(body) == "" {
		delete(e.byName, CustomName)
		e.customSubject = ""
		return nil
	}
	tmpl, subject, err := ParseCustom(body)
	if err != nil {
		return err
	}
	e.byName[CustomName], e.customSubject = tmpl, subject
	return nil
}

// UseConfig registers the config's custom template, if it has one.
func (e *Engine) UseConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	return e.SetCustom(cfg.Options.CustomTemplate)
}

// Render fills the named template with the profile and broker.
func (e *Engine) Render(name string, profile config.Profile, b broker.Broker) (*Email, error) {
	e.mu.RLock()
	tmpl, ok := e.byName[name]
	custom := e.customSubject
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", name)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, newEmailData(name, profile, b, time.Now())); err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	subject, ok := subjects[name]
	switch {
	case name == CustomName && custom != "":
		subject = custom
	case !ok:
		subject = defaultSubject
	}
	return &Email{Subject: subject, Body: body.String()}, nil
}

func newEmailData(name string, p config.Profile, b broker.Broker, now time.Time) EmailData {
	return EmailData{
		FirstName:      p.FirstName,
		LastName:       p.LastName,
		FullName:       p.FullName(),
		DateOfBirth:    p.DateOfBirth,
		Email:          p.Email,
		OtherEmails:    strings.Join(p.AdditionalEmails, ", "),
		Phone:          p.Phone,
		OtherPhones:    strings.Join(p.AdditionalPhones, ", "),
		OtherNames:     strings.Join(p.NameVariants, ", "),
		Address:        p.Address,
		City:           p.City,
		State:          p.State,
		ZipCode:        p.ZipCode,
		Country:        country.Display(p.Country),
		OtherAddresses: strings.Join(p.PreviousAddresses, "; "),
		BrokerName:     b.Name,
		BrokerEmail:    b.Email,
		BrokerWebsite:  b.Website,
		BrokerOptOut:   b.OptOutURL,
		Date:           now.Format("January 2, 2006"),
		Year:           now.Year(),
		Month:          now.Format("January"),
		Template:       name,
	}
}
