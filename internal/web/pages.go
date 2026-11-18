package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/drumandbytes/delistd/internal/evidence"
	"github.com/drumandbytes/delistd/internal/history"
	"github.com/go-chi/chi/v5"
)

// urlID reads a numeric id from the route. A part that is not a plain number
// ("12abc") is not an id.
func urlID(r *http.Request, param string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	return id, err == nil
}

// store returns the history store, or answers 500 itself when there is none.
func (s *Server) store(w http.ResponseWriter, message string) *history.Store {
	if s.historyStore == nil {
		http.Error(w, message, http.StatusInternalServerError)
	}
	return s.historyStore
}

// dashFigure is a headline number that links to the list behind it.
type dashFigure struct {
	Href, Label, Hint string
	Value             int
	Class             string // colour class for the value, if any
}

// ledgerRow is a label and a count on the dashboard's ledgers; a Href makes it a link.
type ledgerRow struct {
	Href, Label, Class string
	Value              int
}

// ringCircumference is that of the coverage ring's r=45 circle.
const ringCircumference = 283

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if cfg := s.getConfig(); cfg == nil || !cfg.HasProfile() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	active := s.activeProfile(r)
	stats := s.getStats(active.ID)
	pipe := s.getPipelineStats(active.ID)

	// a count gets a colour only when it is not zero
	tint := func(n int, class string) string {
		if n > 0 {
			return class
		}
		return ""
	}
	percent := 0
	if stats.TotalBrokers > 0 {
		percent = int(float64(stats.Sent)/float64(stats.TotalBrokers)*100 + 0.5)
	}

	s.renderWithCSRF(w, r, "dashboard.html", map[string]interface{}{
		"Title":         "Dashboard",
		"Profile":       active.Profile,
		"RecentHistory": s.getRecentHistory(active.ID, 10),
		"Stats":         stats,
		"PipelineStats": pipe,
		"Figures": []dashFigure{
			{"/brokers", "Total Brokers", "View all brokers", stats.TotalBrokers, ""},
			{"/history?status=sent", "Requests Sent", "View sent requests", stats.Sent, ""},
			{"/history?status=failed", "Failed", "View failed requests", stats.Failed, tint(stats.Failed, "text-error")},
			{"/brokers?status=pending", "Pending", "View pending brokers", stats.Pending, ""},
		},
		"Ledger": []ledgerRow{
			{"/pipeline?status=awaiting_response", "Waiting for Reply", "", pipe.AwaitingResponse},
			{"/pipeline?status=confirmed", "Removal Confirmed", tint(pipe.Confirmed, "text-success"), pipe.Confirmed},
			{"/pipeline?status=rejected", "Request Rejected", tint(pipe.Rejected, "text-error"), pipe.Rejected},
			{"/pipeline?status=no_data_held", "No data held", "", pipe.NoData},
			{"/pipeline?view=actions", "Action Needed", tint(pipe.PendingTasks, "text-warning"), pipe.PendingTasks},
		},
		"Coverage": []ledgerRow{{"", "Sent", "", stats.Sent}, {"", "Pending", "", stats.Pending}, {"", "Failed", "", stats.Failed}},
		"Percent":  percent,
		"Offset":   ringCircumference - ringCircumference*percent/100,
	})
}

// brokerPageSize is how many broker rows the list renders at a time. All 700+
// rendered twice (table and phone cards) made a 2 MB page that phones took
// seconds to lay out; "Show more" asks for another page.
const brokerPageSize = 50

// pageOfBrokers returns the rows to render for ?limit=, which grows by
// brokerPageSize each time "Show more" is pressed.
func pageOfBrokers(r *http.Request, all []BrokerWithStatus) (rows []BrokerWithStatus, limit int) {
	limit = brokerPageSize
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > limit {
		limit = n
	}
	return all[:min(limit, len(all))], limit
}

// brokerQueryFrom reads the brokers page's filters from the URL.
func brokerQueryFrom(r *http.Request) brokerQuery {
	q := r.URL.Query()
	return brokerQuery{
		Search: q.Get("search"), Category: q.Get("category"), Region: q.Get("region"), Status: q.Get("status"),
		MissingEmail: q.Get("missing_email") == "true", ShowExcluded: q.Get("show_excluded") == "true",
	}
}

// filterOption and filterSelect describe one drop-down of the brokers filter bar.
type filterOption struct {
	Value, Label string
	Selected     bool
}

type filterSelect struct {
	Name, Label, AllLabel, MinWidthClass string
	Options                              []filterOption
}

func options(values []string, current string) []filterOption {
	out := make([]filterOption, len(values))
	for i, v := range values {
		out[i] = filterOption{v, v, v == current}
	}
	return out
}

// brokerFilters is the status, category and region drop-downs, set to the
// query's current values.
func (s *Server) brokerFilters(q brokerQuery) []filterSelect {
	return []filterSelect{
		{Name: "status", Label: "Status", AllLabel: "All Statuses", MinWidthClass: "min-w-30", Options: []filterOption{
			{"pending", "Pending", q.Status == "pending"}, {"sent", "Sent", q.Status == "sent"}, {"failed", "Failed", q.Status == "failed"}}},
		{Name: "category", Label: "Category", AllLabel: "All Categories", MinWidthClass: "min-w-35", Options: options(s.getUniqueCategories(), q.Category)},
		{Name: "region", Label: "Region", AllLabel: "All Regions", MinWidthClass: "min-w-30", Options: options(s.getUniqueRegions(), q.Region)},
	}
}

func (s *Server) handleBrokers(w http.ResponseWriter, r *http.Request) {
	q := brokerQueryFrom(r)
	profileID := s.activeProfile(r).ID
	all := s.listBrokers(profileID, q)
	shown, limit := pageOfBrokers(r, all)
	hidden, country := s.countryHidden(profileID)
	cfg := s.getConfig()

	s.renderWithCSRF(w, r, "brokers.html", map[string]interface{}{
		"Title":         "Data Brokers",
		"Brokers":       shown,
		"Limit":         limit,
		"Filters":       s.brokerFilters(q),
		"Search":        q.Search,
		"Category":      q.Category,
		"Region":        q.Region,
		"Status":        q.Status,
		"MissingEmail":  q.MissingEmail,
		"ShowExcluded":  q.ShowExcluded,
		"Total":         len(s.brokers().Brokers),
		"Filtered":      len(all),
		"CountryHidden": hidden,
		"Country":       country,
		"DailyLimit":    effectiveDailyLimit(cfg),
		"ManualMode":    cfg != nil && cfg.IsManualSend(),
	})
}

// figure is one headline number: the card's label, its value and the colour class of the value.
type figure struct {
	Label, Value, Class string
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("status")
	records := s.getRecentHistory(s.activeProfile(r).ID, 1000)
	if filter == "sent" || filter == "failed" {
		var kept []history.Record
		for _, rec := range records {
			if string(rec.Status) == filter {
				kept = append(kept, rec)
			}
		}
		records = kept
	}

	sent := 0
	for _, rec := range records {
		if rec.Status == history.StatusSent {
			sent++
		}
	}
	failed := len(records) - sent
	rate := 0
	if len(records) > 0 {
		rate = (sent*100 + len(records)/2) / len(records) // rounded
	}

	tabs := []struct{ Label, Href, Key string }{{"All", "/history", ""}, {"Sent", "/history?status=sent", "sent"}, {"Failed", "/history?status=failed", "failed"}}
	s.renderWithCSRF(w, r, "history.html", map[string]interface{}{
		"Title":        "History",
		"History":      records,
		"StatusFilter": filter,
		"Tabs":         tabs,
		"Figures": []figure{
			{"Total Requests", strconv.Itoa(len(records)), "text-primary"},
			{"Successful", strconv.Itoa(sent), "text-success"},
			{"Failed", strconv.Itoa(failed), "text-error"},
			{"Success Rate", strconv.Itoa(rate) + "%", "text-primary"},
		},
	})
}

// PipelineStats are the pipeline dashboard's counters.
type PipelineStats struct {
	EmailSent            int
	AwaitingResponse     int
	FormRequired         int
	FormFilled           int
	AwaitingCaptcha      int
	CaptchaSolved        int
	AwaitingConfirmation int
	Confirmed            int
	NoData               int
	Rejected             int
	Failed               int
	PendingTasks         int
	NeedsReview          int
}

func (s *Server) getPipelineStats(profileID string) PipelineStats {
	stats, _, _ := s.loadPipelineStats(profileID)
	return stats
}

func (s *Server) handleResponseReview(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "responseID")
	if !ok {
		http.Error(w, "Response not found", http.StatusNotFound)
		return
	}
	store := s.store(w, "Database not available")
	if store == nil {
		return
	}
	response, err := store.GetBrokerResponseByID(id, s.activeProfile(r).ID)
	if err != nil || response == nil {
		http.Error(w, "Response not found", http.StatusNotFound)
		return
	}
	s.renderWithCSRF(w, r, "response-review.html", map[string]interface{}{"Title": "Review Response", "Response": response})
}

// handleExport downloads the active profile's evidence report, the same one
// `delistd export` writes (?format=html|json, optional ?since=YYYY-MM-DD).
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}
	if format != "html" && format != "json" {
		http.Error(w, "format must be html or json", http.StatusBadRequest)
		return
	}
	var since time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			http.Error(w, "since must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		since = t
	}
	store := s.store(w, "History database not available")
	if store == nil {
		return
	}

	profile := s.activeProfile(r)
	requests, err := store.GetAllRequests(profile.ID)
	if err != nil {
		http.Error(w, "Failed to read requests: "+err.Error(), http.StatusInternalServerError)
		return
	}
	responses, err := store.GetBrokerResponsesForExport(profile.ID)
	if err != nil {
		http.Error(w, "Failed to read responses: "+err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now()
	report := evidence.Build(profile.Profile, requests, responses, s.brokers(), s.tmplEngine, since, now)

	contentType := "text/html; charset=utf-8"
	var body []byte
	if format == "json" {
		contentType = "application/json"
		body, err = json.MarshalIndent(report, "", "  ")
	} else {
		body, err = evidence.RenderHTML(report)
	}
	if err != nil {
		http.Error(w, "Failed to render report: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="delistd-evidence-%s-%s.%s"`, profile.ID, now.Format("2006-01-02"), format))
	_, _ = w.Write(body)
}
