package web

import (
	"fmt"
	"html"
	"html/template"
)

// Outline icons (24x24 stroke, Heroicons) drawn the same way everywhere, so a
// template says {{icon "check" "w-4 h-4"}} instead of repeating the path data.
var iconPaths = map[string][]string{
	"refresh":              {`M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15`},
	"send":                 {`M12 19l9 2-9-18-9 18 9-2zm0 0v-8`},
	"info":                 {`M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z`},
	"search":               {`M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z`},
	"warning":              {`M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z`},
	"clock":                {`M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z`},
	"chevron-left":         {`M15 19l-7-7 7-7`},
	"download":             {`M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4`},
	"trash":                {`M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16`},
	"document-text":        {`M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z`},
	"exclamation-circle":   {`M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z`},
	"clipboard":            {`M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2`},
	"desktop":              {`M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z`},
	"sun":                  {`M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z`},
	"moon":                 {`M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z`},
	"menu":                 {`M4 6h16M4 12h16M4 18h16`},
	"check":                {`M5 13l4 4L19 7`},
	"close":                {`M6 18L18 6M6 6l12 12`},
	"shield-check":         {`M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z`},
	"external-link":        {`M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14`},
	"link":                 {`M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101`},
	"mail":                 {`M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z`},
	"cog":                  {`M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z`, `M15 12a3 3 0 11-6 0 3 3 0 016 0z`},
	"arrow-left":           {`M10 19l-7-7m0 0l7-7m-7 7h18`},
	"users":                {`M17 20h5v-2a4 4 0 00-3-3.87M9 20H4v-2a4 4 0 013-3.87m6-1.13a4 4 0 10-4-4 4 4 0 004 4zm6-2a4 4 0 11-8 0 4 4 0 018 0z`},
	"plus":                 {`M12 4v16m8-8H4`},
	"pencil":               {`M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z`},
	"inbox":                {`M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4`},
	"lock":                 {`M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z`},
	"sliders":              {`M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4`},
	"chevron-right":        {`M9 5l7 7-7 7`},
	"check-circle":         {`M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z`},
	"arrow-right":          {`M13 7l5 5m0 0l-5 5m5-5H6`},
	"building":             {`M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4`},
	"clipboard-copy":       {`M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z`},
	"chevron-double-right": {`M13 5l7 7-7 7M5 5l7 7-7 7`},
	"eye":                  {`M15 12a3 3 0 11-6 0 3 3 0 016 0z`, `M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z`},
}

// icon renders one of iconPaths with the given CSS classes. An unknown name
// renders nothing.
func icon(name, class string) template.HTML {
	if name == "github" {
		return template.HTML(`<svg class="` + html.EscapeString(class) + `" viewBox="0 0 16 16" aria-hidden="true" fill="currentColor"><path d="` + githubPath + `"/></svg>`)
	}
	paths, ok := iconPaths[name]
	if !ok {
		return ""
	}
	svg := fmt.Sprintf(`<svg class="%s" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">`, html.EscapeString(class))
	for _, d := range paths {
		svg += `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="` + d + `"/>`
	}
	return template.HTML(svg + "</svg>")
}

// spinner is the animated busy indicator HTMX shows while a request runs; id
// lets hx-indicator point at it.
func spinner(id, class string) template.HTML {
	return template.HTML(fmt.Sprintf(`<svg id="%s" class="htmx-indicator %s animate-spin" fill="none" viewBox="0 0 24 24" aria-hidden="true">`+
		`<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>`+
		`<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>`,
		html.EscapeString(id), html.EscapeString(class)))
}

// card opens a card, with a heading when title is not empty; {{endcard}}
// closes it. A card is the page's basic block: a bordered panel with padding.
func card(title string, class ...string) template.HTML {
	pad := "p-6"
	if len(class) > 0 {
		pad = class[0]
	}
	open := `<div class="card ` + html.EscapeString(pad) + `">`
	if title != "" {
		open += `<h2 class="font-display text-lg font-semibold text-primary mb-4">` + html.EscapeString(title) + `</h2>`
	}
	return template.HTML(open)
}

// dict builds a map from alternating keys and values, so a partial can take
// several named arguments: {{template "partials/x.html" (dict "A" 1 "B" 2)}}.
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments")
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key %v is not a string", pairs[i])
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// section opens a settings-style card: an icon beside a heading and an
// optional line under it, then {{endcard}}. id makes the card a link target.
func section(iconName, title, subtitle string, id ...string) template.HTML {
	attr := ""
	if len(id) > 0 && id[0] != "" {
		attr = ` id="` + html.EscapeString(id[0]) + `"`
	}
	heading := `<h2 class="font-display text-lg font-semibold text-primary">` + html.EscapeString(title) + `</h2>`
	if subtitle != "" {
		heading = "<div>" + heading + `<p class="text-sm text-muted">` + html.EscapeString(subtitle) + `</p></div>`
	}
	return template.HTML(`<div class="card p-6"` + attr + `><div class="flex items-center gap-3 mb-6">` +
		string(icon(iconName, "w-5 h-5 text-muted flex-shrink-0")) + heading + `</div>`)
}

// badge is a small status label: kind is success, warning, error or neutral.
func badge(kind, text string) template.HTML {
	return template.HTML(`<span class="badge badge-` + html.EscapeString(kind) + `">` + html.EscapeString(text) + `</span>`)
}

// githubPath is the GitHub mark, which is filled, not drawn with a stroke like the others.
const githubPath = `M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0016 8c0-4.42-3.58-8-8-8z`
