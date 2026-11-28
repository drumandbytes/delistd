package web

import (
	"slices"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/history"
)

// Stats are the dashboard's headline numbers.
type Stats struct {
	TotalBrokers int
	Sent         int
	Failed       int
	Pending      int
}

// BrokerWithStatus is a broker plus what the active profile's history says
// about it.
type BrokerWithStatus struct {
	broker.Broker
	Status     string // "never", "sent" or "failed"
	LastSent   string // formatted date, empty when never sent
	TotalSent  int
	Excluded   bool // excluded by the config's ExcludedBrokers or ExcludedCategories
	ManualMode bool // send_mode is manual: rows offer "Email" and "Mark sent" instead of "Send"
	lastSentAt time.Time
}

// brokerQuery narrows the broker list. Text fields are matched
// case-insensitively and may be empty.
type brokerQuery struct {
	Search, Category, Region string
	// Status is one of never/pending, sent, failed, eligible (not sent within
	// the resend cooldown); anything else, including "all", keeps everyone.
	Status       string
	Only, Skip   []string // broker IDs: when Only is set nothing else is listed; Skip marks IDs excluded for this run
	MissingEmail bool     // only brokers with no contact address
	ShowExcluded bool     // keep excluded brokers, with Excluded set, instead of dropping them
}

func lowerSet(items []string, trim bool) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		if trim {
			it = strings.TrimSpace(it)
		}
		set[strings.ToLower(it)] = true
	}
	return set
}

func clean(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// listBrokers applies q the same way the CLI's broker filter does: config
// exclusions, the profile's country and the configured regions all count.
func (s *Server) listBrokers(profileID string, q brokerQuery) []BrokerWithStatus {
	var statuses map[string]history.BrokerStatus
	if s.historyStore != nil {
		statuses, _ = s.historyStore.GetAllBrokerStatuses(profileID)
	}

	var excludedKeys, excludedCategories, regions map[string]bool // nil sets match nothing
	var country string
	manual := false
	if cfg := s.getConfig(); cfg != nil {
		if p, err := cfg.GetProfile(profileID); err == nil {
			country = p.Country
		}
		excludedKeys = lowerSet(cfg.Options.ExcludedBrokers, false) // by ID or by name
		excludedCategories = lowerSet(cfg.Options.ExcludedCategories, true)
		regions = broker.Set(cfg.Options.Regions)
		manual = cfg.IsManualSend()
	}

	search, category, region, status := clean(q.Search), clean(q.Category), clean(q.Region), clean(q.Status)
	only, skip := broker.Set(q.Only), broker.Set(q.Skip)

	now := time.Now()
	var out []BrokerWithStatus
	for _, b := range s.brokers().Brokers {
		id := strings.ToLower(b.ID)
		if len(only) > 0 && !only[id] {
			continue
		}
		excluded := excludedKeys[id] || excludedKeys[strings.ToLower(b.Name)] ||
			excludedCategories[strings.ToLower(b.Category)] || skip[id]
		if excluded && !q.ShowExcluded {
			continue
		}
		if search != "" && !containsAnyFold(search, id, b.Name, b.Email) {
			continue
		}
		if category != "" && strings.ToLower(b.Category) != category {
			continue
		}
		if !b.AppliesTo(country) || !b.InRegions(regions) {
			continue
		}
		if region != "" && strings.ToLower(b.Region) != region {
			continue
		}
		if q.MissingEmail && b.Email != "" {
			continue
		}

		row := BrokerWithStatus{Broker: b, Status: "never", Excluded: excluded, ManualMode: manual}
		var st *history.BrokerStatus
		if h, ok := statuses[b.ID]; ok {
			st = &h
			row.Status, row.TotalSent, row.lastSentAt = string(h.Status), h.TotalSent, h.LastSent
			if !h.LastSent.IsZero() {
				row.LastSent = h.LastSent.Format("Jan 2, 2006")
			}
		}
		if !history.MatchesFilter(status, st, now) {
			continue
		}
		out = append(out, row)
	}
	return out
}

// containsAnyFold reports whether needle (already lowercase) is in any field.
func containsAnyFold(needle string, fields ...string) bool {
	return slices.ContainsFunc(fields, func(f string) bool { return strings.Contains(strings.ToLower(f), needle) })
}

// countryHidden counts the brokers the profile's country rules out, for the
// "N hidden" note on the brokers page; zero when no country is set.
func (s *Server) countryHidden(profileID string) (n int, country string) {
	cfg := s.getConfig()
	if cfg == nil {
		return 0, ""
	}
	p, err := cfg.GetProfile(profileID)
	if err != nil || p.Country == "" {
		return 0, ""
	}
	for _, b := range s.brokers().Brokers {
		if !b.AppliesTo(p.Country) {
			n++
		}
	}
	return n, p.Country
}

func (s *Server) uniqueValues(get func(broker.Broker) string) []string {
	seen := map[string]bool{}
	var out []string
	for _, b := range s.brokers().Brokers {
		if v := get(b); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) getUniqueCategories() []string {
	return s.uniqueValues(func(b broker.Broker) string { return b.Category })
}

func (s *Server) getUniqueRegions() []string {
	return s.uniqueValues(func(b broker.Broker) string { return b.Region })
}

func (s *Server) getStats(profileID string) Stats {
	st := Stats{TotalBrokers: len(s.brokers().Brokers)}
	if s.historyStore != nil {
		if _, sent, failed, err := s.historyStore.GetStats(profileID); err == nil {
			st.Sent, st.Failed = sent, failed
		}
	}
	st.Pending = max(st.TotalBrokers-st.Sent-st.Failed, 0)
	return st
}

func (s *Server) getRecentHistory(profileID string, limit int) []history.Record {
	if s.historyStore == nil {
		return nil
	}
	records, _ := s.historyStore.GetRecentRequests(profileID, limit)
	return records
}
