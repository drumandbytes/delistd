package web

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/history"
)

var updateSnapshots = flag.Bool("update-snapshots", false, "rewrite testdata/pages_snapshot.json.gz from the current output")

// pageFingerprint is what a person sees and can act on in a page: its text,
// links, forms and fields. Markup, classes and ids are left out, so templates
// can be restructured freely while this stays put.
type pageFingerprint struct {
	Status   int
	Location string   `json:",omitempty"`
	Type     string   `json:",omitempty"`
	Text     string   `json:",omitempty"`
	Links    []string `json:",omitempty"`
	Forms    []string `json:",omitempty"`
	Raw      string   `json:",omitempty"` // non-HTML bodies, as is
}

func fingerprint(rec *httptest.ResponseRecorder) pageFingerprint {
	body := rec.Body.String()
	// judged from the body, not the header, so the snapshot does not care
	// whether a handler sets Content-Type or leaves it to sniffing
	ct := http.DetectContentType(rec.Body.Bytes())
	if h := rec.Header().Get("Content-Type"); strings.HasPrefix(h, "application/json") || strings.HasPrefix(h, "text/plain") {
		ct = h
	}
	fp := pageFingerprint{Status: rec.Code, Location: rec.Header().Get("Location"), Type: strings.Split(ct, ";")[0]}
	if fp.Type != "text/html" {
		if len(body) < 4000 {
			fp.Raw = body
		}
		return fp
	}
	links := map[string]bool{}
	forms := map[string]bool{}
	var text []string
	var walk func(n *html.Node, skip bool, form string)
	walk = func(n *html.Node, skip bool, form string) {
		switch n.Type {
		case html.TextNode:
			if !skip {
				if t := strings.Join(strings.Fields(n.Data), " "); t != "" {
					text = append(text, t)
				}
			}
		case html.ElementNode:
			attr := func(k string) string {
				for _, a := range n.Attr {
					if a.Key == k {
						return a.Val
					}
				}
				return ""
			}
			switch n.Data {
			case "script", "style", "svg", "head":
				skip = true
			case "a":
				if h := attr("href"); h != "" {
					links[h] = true
				}
			case "form":
				form = strings.ToUpper(attr("method")) + " " + attr("action")
				forms[form] = true
			case "input", "select", "textarea", "button":
				if name := attr("name"); name != "" && attr("type") != "hidden" {
					forms[form+" field "+name+" "+attr("type")] = true
				}
				if h := attr("hx-get") + attr("hx-post") + attr("hx-delete"); h != "" {
					links["hx:"+h] = true
				}
			}
			for _, k := range []string{"hx-get", "hx-post", "hx-delete"} {
				if h := attr(k); h != "" && n.Data != "input" && n.Data != "select" && n.Data != "textarea" && n.Data != "button" {
					links["hx:"+h] = true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skip, form)
		}
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err == nil {
		// the page's own content: the nav, theme switch and footer belong to
		// the layout and change with it
		root := doc
		var find func(n *html.Node)
		find = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "main" {
				root = n
				return
			}
			for c := n.FirstChild; c != nil && root == doc; c = c.NextSibling {
				find(c)
			}
		}
		find(doc)
		walk(root, false, "")
	}
	fp.Text = undate(strings.Join(text, " | "))
	for l := range links {
		fp.Links = append(fp.Links, undate(l)) // the mailto: body carries today's date
	}
	for f := range forms {
		fp.Forms = append(fp.Forms, f)
	}
	sort.Strings(fp.Links)
	sort.Strings(fp.Forms)
	return fp
}

// snapshotServer is a server with enough variety in its data to light up
// every branch of the page templates: brokers of several kinds, sent, failed
// and manual history, a reply of every type, and tasks in every state.
func snapshotServer(t *testing.T) (*Server, map[string]int64) {
	t.Helper()
	time.Local = time.UTC
	s := smokeServer(t)
	s.brokers().Brokers = nil
	cats := []string{"people-search", "marketing", "credit", "requires-id"}
	regions := []string{"us", "eu", "global"}
	for i := 0; i < 14; i++ {
		b := broker.Broker{ID: fmt.Sprintf("b%02d", i), Name: fmt.Sprintf("Broker %02d", i), Region: regions[i%3], Category: cats[i%4]}
		if i%5 != 4 {
			b.Email = fmt.Sprintf("privacy@b%02d.example", i)
		} else {
			b.OptOutURL = fmt.Sprintf("https://b%02d.example/opt-out", i)
		}
		if i == 7 {
			b.Countries = []string{"DE", "FR"}
		}
		s.brokers().Brokers = append(s.brokers().Brokers, b)
	}
	st := s.historyStore
	base := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	ids := map[string]int64{}
	for i := 0; i < 14; i++ {
		status := history.StatusSent
		errText := ""
		switch {
		case i%7 == 3:
			status, errText = history.StatusFailed, "550 mailbox unavailable"
		case i > 10:
			continue
		}
		rec := &history.Record{ProfileID: "default", BrokerID: fmt.Sprintf("b%02d", i), BrokerName: fmt.Sprintf("Broker %02d", i), Email: fmt.Sprintf("privacy@b%02d.example", i),
			Template: "gdpr", Status: status, Error: errText, SentAt: base.Add(time.Duration(i) * 24 * time.Hour)}
		if i == 2 {
			rec.SentMethod = "manual"
		}
		if err := st.Add(rec); err != nil {
			t.Fatal(err)
		}
	}
	types := []string{"success", "pending", "form_required", "confirmation_required", "rejected", "no_data_held", "bounced", "unknown"}
	for i, ty := range types {
		resp := &history.BrokerResponse{ProfileID: "default", BrokerID: fmt.Sprintf("b%02d", i), BrokerName: fmt.Sprintf("Broker %02d", i), ResponseType: ty,
			EmailFrom: fmt.Sprintf("privacy@b%02d.example", i), EmailSubject: "Re: Personal Data Removal Request (" + ty + ")",
			EmailBody: "Hello,\nThis is the " + ty + " reply body.\nhttps://b0" + fmt.Sprint(i) + ".example/optout", Confidence: 0.5 + float64(i)/20,
			NeedsReview: i%3 == 0, ReceivedAt: base.Add(time.Duration(i) * time.Hour), ProcessedAt: base.Add(time.Duration(i) * time.Hour)}
		if ty == "form_required" {
			resp.FormURL = "https://b02.example/optout"
		}
		if ty == "confirmation_required" {
			resp.ConfirmURL = "https://b03.example/confirm?token=abc"
		}
		if err := st.AddBrokerResponse(resp); err != nil {
			t.Fatal(err)
		}
		ids["response:"+ty] = resp.ID
	}
	for i, tt := range []struct {
		typ    history.TaskType
		status string
		note   string
	}{{history.TaskCaptcha, "pending", ""}, {history.TaskManualForm, "pending", "fill this in"}, {history.TaskReview, "pending", "check the reply"}, {history.TaskManualForm, "completed", ""}, {history.TaskCaptcha, "skipped", ""}} {
		task := &history.PendingTask{ProfileID: "default", BrokerID: fmt.Sprintf("b%02d", i), BrokerName: fmt.Sprintf("Broker %02d", i), TaskType: tt.typ,
			FormURL: fmt.Sprintf("https://b%02d.example/opt-out", i), Notes: tt.note, Status: tt.status, BrowserState: `{"first_name":"Test"}`, CreatedAt: base}
		if err := st.AddPendingTask(task); err != nil {
			t.Fatal(err)
		}
		ids[fmt.Sprintf("task:%d", i)] = task.ID
	}
	for i, ps := range []history.PipelineStatus{history.PipelineAwaitingResponse, history.PipelineConfirmed, history.PipelineRejected, history.PipelineFormRequired, history.PipelineNoData} {
		_ = st.UpdatePipelineStatus("default", fmt.Sprintf("b%02d", i), ps)
	}
	return s, ids
}

// snapshotPaths returns request paths with {name} placeholders for row ids,
// which differ between runs only if the seeding order does.
func snapshotPaths() []string {
	return []string{
		"/", "/brokers", "/brokers?status=sent", "/brokers?status=failed", "/brokers?status=pending", "/brokers?category=marketing",
		"/brokers?region=eu", "/brokers?search=broker+03", "/brokers?missing_email=true", "/brokers?show_excluded=true", "/brokers?limit=5",
		"/api/brokers", "/api/brokers?status=sent", "/api/brokers?search=zzz", "/api/brokers?limit=100&category=credit",
		"/api/brokers/b01/status", "/api/brokers/nope/status",
		"/brokers/new", "/brokers/b01/email", "/brokers/nope/email",
		"/history", "/history?status=sent", "/history?status=failed",
		"/settings", "/settings/profiles/new", "/settings/profiles/default/edit", "/settings/profiles/nope/edit",
		"/pipeline", "/pipeline?status=confirmed", "/pipeline?status=awaiting_response", "/pipeline?view=actions", "/pipeline?type=pending", "/pipeline?needs_review=true", "/pipeline?search=ticket",
		"/api/pipeline/responses", "/api/pipeline/responses?type=unknown", "/api/pipeline/responses?needs_review=true",
		"/pipeline/responses/{response:unknown}", "/pipeline/responses/99999",
		"/automation", "/tasks", "/tasks?type=captcha", "/tasks?status=completed", "/forms",
		"/tasks/{task:1}", "/tasks/{task:1}/helper", "/tasks/{task:0}/helper", "/tasks/{task:2}", "/tasks/99999",
		"/setup/", "/setup/profile", "/setup/email", "/setup/test", "/setup/complete", "/unlock",
		"/export", "/api/job/active", "/oauth/status", "/nope",
	}
}

func TestPagesMatchSnapshot(t *testing.T) {
	s, ids := snapshotServer(t)
	router := s.setupRouter()
	got := map[string]pageFingerprint{}
	for _, key := range snapshotPaths() {
		path := key
		for name, id := range ids {
			path = strings.ReplaceAll(path, "{"+name+"}", fmt.Sprint(id))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, loopbackRequest(http.MethodGet, path, nil))
		got[key] = fingerprint(rec)
	}
	file := filepath.Join("testdata", "pages_snapshot.json.gz")
	if *updateSnapshots {
		b, _ := json.MarshalIndent(got, "", " ")
		var zb bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&zb, 9)
		_, _ = zw.Write(b)
		_ = zw.Close()
		if err := os.WriteFile(file, zb.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]pageFingerprint
	if err := json.NewDecoder(zr).Decode(&want); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for p := range want {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	bad := 0
	for _, p := range paths {
		if p == "/settings" && runtime.GOOS == "windows" {
			continue // the automation section differs: Windows has no OS scheduler
		}
		g, w := got[p], want[p]
		if fmt.Sprint(g) != fmt.Sprint(w) {
			if bad++; bad <= 6 {
				t.Errorf("%s differs\n got  %s\n want %s", p, clip(fmt.Sprintf("%+v", g)), clip(fmt.Sprintf("%+v", w)))
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d pages differ from the snapshot (run with -update-snapshots if that is intended)", bad, len(paths))
	}
}

// undate turns "now" in a page into a fixed word: rows the store stamps with
// the current time would otherwise change the snapshot every minute.
func undate(s string) string {
	today := time.Now().Format("Jan 2, 2006")
	s = regexp.MustCompile(regexp.QuoteMeta(today)+` \d{1,2}:\d{2} [AP]M`).ReplaceAllString(s, "<now>")
	s = regexp.MustCompile(`Generated \d{1,2} \w+ \d{4}, \d{2}:\d{2} UTC`).ReplaceAllString(s, "Generated <now>") // evidence report
	s = strings.ReplaceAll(s, today, "<today>")
	long := time.Now().Format("January 2, 2006") // template {{.Date}}
	s = strings.ReplaceAll(s, long, "<today>")
	encoded := strings.ReplaceAll(url.QueryEscape(long), "+", "%20") // the same date inside a mailto: link
	return strings.ReplaceAll(s, encoded, "<today>")
}

func clip(s string) string {
	if len(s) > 1500 {
		return s[:1500] + "..."
	}
	return s
}
