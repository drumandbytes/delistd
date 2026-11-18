package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/country"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

var templateFuncs = template.FuncMap{
	"pipelineLabel": pipelineLabel,
	"formatTime":    func(t time.Time) string { return t.Format("Jan 2, 2006 3:04 PM") },
	"formatDate":    func(t time.Time) string { return t.Format("Jan 2, 2006") },
	"providerName":  config.ProviderName,
	"countries":     country.All,
	"countryCode": func(name string) string {
		code, _ := country.Code(name)
		return code
	},
	"icon":        icon,
	"card":        card,
	"section":     section,
	"badge":       badge,
	"endcard":     func() template.HTML { return "</div>" },
	"dict":        dict,
	"profileForm": profileFormFor,
	"spinner":     spinner,
	"add":         func(a, b int) int { return a + b },
	"sub":         func(a, b int) int { return a - b },
	// percent shows a 0..1 confidence as a whole-number percentage
	"percent": func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
}

const templateDir = "templates"

// loadTemplates parses every template once. The layout and the partials form
// a shared base; each page is a clone of it plus the page's own "content"
// block (so pages can all define "content" without clashing), and each
// partial can also be rendered alone, as an HTMX fragment.
func loadTemplates() (map[string]*template.Template, error) {
	base := template.New("").Funcs(templateFuncs)

	layout, err := templatesFS.ReadFile(templateDir + "/layout.html")
	if err != nil {
		return nil, fmt.Errorf("read layout: %w", err)
	}
	if _, err := base.Parse(string(layout)); err != nil {
		return nil, fmt.Errorf("parse layout: %w", err)
	}

	var pages, partials []string // paths under templates/
	err = fs.WalkDir(templatesFS, templateDir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() || !strings.HasSuffix(path, ".html") || path == templateDir+"/layout.html":
		case strings.Contains(path, "/partials/"):
			partials = append(partials, path)
		default:
			pages = append(pages, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}

	// a partial is invoked as {{template "partials/x.html" .}}
	for _, path := range partials {
		src, err := templatesFS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if _, err := base.New(strings.TrimPrefix(path, templateDir+"/")).Parse(string(src)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	set := make(map[string]*template.Template, len(pages)+len(partials))
	for _, path := range partials {
		name := strings.TrimPrefix(path, templateDir+"/")
		set[name] = base.Lookup(name)
	}
	for _, path := range pages {
		src, err := templatesFS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		page, err := base.Clone()
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(path, templateDir+"/")
		if _, err := page.Parse(string(src)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		set[name] = page
	}
	return set, nil
}

// writeHTML sends a rendered page. Rendering into a buffer first means a
// template error gives a clean 500 instead of half a page followed by an
// error message.
func writeHTML(w http.ResponseWriter, t *template.Template, entry string, data any) {
	var buf bytes.Buffer
	var err error
	if entry == "" {
		err = t.Execute(&buf, data)
	} else {
		err = t.ExecuteTemplate(&buf, entry, data)
	}
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	t, ok := s.templates[name]
	if !ok {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}
	writeHTML(w, t, "", data)
}

func (s *Server) renderWithCSRF(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	// the CSRF check is Sec-Fetch-Site based and uses no tokens; the keys stay
	// so a template mentioning {{.CSRFField}} still renders
	data["Nav"] = navItems
	data["CSRFToken"] = ""
	data["CSRFField"] = template.HTML("")

	// always set, even before any config exists (the first /setup visit):
	// the layout takes len .Profiles
	data["Profiles"] = []config.NamedProfile{}
	data["ActiveProfile"] = config.NamedProfile{}
	data["CurrentPath"] = r.URL.Path
	data["Version"] = Version
	data["UpgradeNotice"] = s.upgradeNotice()
	if cfg := s.getConfig(); cfg != nil {
		data["Profiles"] = cfg.GetProfiles()
		data["ActiveProfile"] = s.activeProfile(r)
	}

	t, ok := s.templates[name]
	if !ok {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}
	writeHTML(w, t, "layout", data)
}

// navItems are the main navigation links. Title is the page title that marks
// a link as the current one.
var navItems = []struct{ Href, Label, Title string }{
	{"/", "Dashboard", "Dashboard"},
	{"/brokers", "Brokers", "Data Brokers"},
	{"/pipeline", "Pipeline", "Pipeline"},
	{"/history", "History", "History"},
	{"/settings", "Settings", "Settings"},
}
