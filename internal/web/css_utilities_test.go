package web

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssClass   = regexp.MustCompile(`\.((?:\\.|[A-Za-z0-9_-])+)`)
	cssPrelude = regexp.MustCompile(`([^{}]+)\{`)
)

// cssSelectors returns the class names a stylesheet defines rules for.
func cssSelectors(t *testing.T, file string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("static", "css", file))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	// only the selector before each "{", so values like 0.5s or .75rem are not classes
	for _, prelude := range cssPrelude.FindAllStringSubmatch(cssComment.ReplaceAllString(string(b), ""), -1) {
		if strings.HasPrefix(strings.TrimSpace(prelude[1]), "@") {
			continue
		}
		for _, m := range cssClass.FindAllStringSubmatch(prelude[1], -1) {
			out[strings.ReplaceAll(m[1], `\`, "")] = true
		}
	}
	return out
}

// markupSource is everything that can name a CSS class: templates, the Go that
// builds class strings, and the page scripts, with comments left out.
func markupSource(t *testing.T) string {
	t.Helper()
	var sb strings.Builder
	goComment := regexp.MustCompile(`(?m)^\s*//.*$`)
	tmplComment := regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}|<!--.*?-->`)
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, _ := os.ReadFile(path)
		switch {
		case strings.HasSuffix(path, ".html"):
			sb.WriteString(tmplComment.ReplaceAllString(string(b), ""))
		case strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".js"):
			sb.WriteString(goComment.ReplaceAllString(string(b), ""))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func classUsed(src, class string) bool {
	return regexp.MustCompile(`(?:^|[^\w-])` + regexp.QuoteMeta(class) + `(?:$|[^\w-])`).MatchString(src)
}

// A utility class a template uses must have a rule: without one the element
// silently loses its border, spacing or colour and nothing else notices.
func TestTemplateUtilityClassesHaveRules(t *testing.T) {
	selectors := map[string]bool{}
	for _, f := range []string{"app.css", "utilities.css", "tokens.css"} {
		for c := range cssSelectors(t, f) {
			selectors[c] = true
		}
	}
	// Class names built from utilities: spacing, sizing, flex/grid, colour, border and text.
	utility := regexp.MustCompile(`^(?:[a-z]+:)*-?(?:p[xytblr]?|m[xytblr]?|gap(?:-[xy])?|space-[xy]|w|h|min-[wh]|max-[wh]|text|bg|border(?:-[tblr])?|divide(?:-[xy])?|rounded|flex|grid|col-span|items|justify|self|order|overflow|opacity|font|leading|tracking|z|top|left|right|bottom|inset|ring)(?:-[\w./\[\]()%#,-]+)?$`)
	attr := regexp.MustCompile(`class="([^"]*)"`)
	var missing []string
	seen := map[string]bool{}
	err := filepath.WalkDir("templates", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		b, _ := os.ReadFile(path)
		for _, m := range attr.FindAllStringSubmatch(string(b), -1) {
			for _, tok := range strings.Fields(m[1]) {
				if strings.Contains(tok, "{{") || strings.Contains(tok, "}}") || seen[tok] || !utility.MatchString(tok) {
					continue
				}
				seen[tok] = true
				if !selectors[tok] {
					missing = append(missing, tok+" ("+filepath.Base(path)+")")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// class strings built in Go (chips, buttons, row tints, widths)
	str := regexp.MustCompile("\"([^\"\\n]*)\"|`([^`]*)`")
	err = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, _ := os.ReadFile(path)
		for _, m := range str.FindAllStringSubmatch(string(b), -1) {
			for _, tok := range strings.Fields(m[1] + m[2]) {
				if seen[tok] || !strings.Contains(tok, "-") || strings.HasSuffix(tok, "-src") || !utility.MatchString(tok) || strings.ContainsAny(tok, "{}<>=/\\") && !strings.HasPrefix(tok, "-") {
					continue
				}
				seen[tok] = true
				if !selectors[tok] && regexp.MustCompile(`^[a-z0-9:.\[\]()%-]+$`).MatchString(tok) {
					missing = append(missing, tok+" ("+filepath.Base(path)+")")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("classes used in templates with no CSS rule:\n  %s", strings.Join(missing, "\n  "))
	}
}

// And the other way: a rule nothing uses is dead weight that only grows.
func TestStylesheetClassesAreUsed(t *testing.T) {
	src := markupSource(t)
	// state classes the page scripts toggle or that HTMX adds
	allowed := map[string]bool{"light": true, "htmx-request": true, "htmx-indicator": true, "current": true, "done": true, "active": true, "selected": true}
	var unused []string
	for _, f := range []string{"app.css", "utilities.css"} {
		for c := range cssSelectors(t, f) {
			if !allowed[c] && !classUsed(src, c) {
				unused = append(unused, c+" ("+f+")")
			}
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Errorf("CSS rules for classes no template, script or Go code uses:\n  %s", strings.Join(unused, "\n  "))
	}
}

// Styling lives in the stylesheets. The one inline style left is a value the
// server computes (the coverage ring's dash offset).
func TestTemplatesHaveNoInlineStyles(t *testing.T) {
	err := filepath.WalkDir("templates", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		b, _ := os.ReadFile(path)
		for _, m := range regexp.MustCompile(`style="[^"]*"`).FindAllString(string(b), -1) {
			if !strings.HasPrefix(m, `style="stroke-dashoffset:`) {
				t.Errorf("%s: inline %s - add a class to app.css instead", path, m)
			}
		}
		if strings.Contains(string(b), "<style") {
			t.Errorf("%s: a <style> block - move it to app.css", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A spinner nested in a plain wrapper span (the bulk-send and retry buttons) is
// an inline box, where width and height do nothing and only a border fragment
// shows (#313). It has to size itself.
func TestSpinnerSizesItself(t *testing.T) {
	css, err := os.ReadFile("static/css/app.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`(?m)^\.spinner \{([^}]*)\}`).FindSubmatch(css)
	if rule == nil {
		t.Fatal(".spinner rule not found in app.css")
	}
	if !strings.Contains(string(rule[1]), "display: inline-block") {
		t.Errorf(".spinner needs display: inline-block, got: %s", rule[1])
	}
}
