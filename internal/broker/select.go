package broker

import (
	"slices"
	"strings"

	"github.com/drumandbytes/delistd/internal/country"
)

// Set lower-cases and trims items into a lookup set, skipping empty ones.
func Set(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, s := range items {
		if key := strings.ToLower(strings.TrimSpace(s)); key != "" {
			set[key] = true
		}
	}
	return set
}

// AppliesTo reports whether a request from someone in profileCountry (an ISO
// code, or a legacy country name) is in scope for the broker. A broker without
// a countries list applies to everyone, and an empty or unrecognised country
// never hides anything.
func (b Broker) AppliesTo(profileCountry string) bool {
	if len(b.Countries) == 0 {
		return true
	}
	code, ok := country.Code(profileCountry)
	if !ok {
		return true
	}
	return slices.ContainsFunc(b.Countries, func(c string) bool { return strings.EqualFold(strings.TrimSpace(c), code) })
}

// ForCountry keeps the brokers that apply to profileCountry.
func ForCountry(brokers []Broker, profileCountry string) []Broker {
	kept := make([]Broker, 0, len(brokers))
	for _, b := range brokers {
		if b.AppliesTo(profileCountry) {
			kept = append(kept, b)
		}
	}
	return kept
}

// InRegions reports whether b is in scope for a configured region set (lower-case
// keys). An empty set, or one with "global", means every broker, and global
// brokers are in scope for any set.
func (b Broker) InRegions(set map[string]bool) bool {
	if len(set) == 0 || set["global"] {
		return true
	}
	r := strings.ToLower(b.Region)
	return r == "global" || set[r]
}

// Filter keeps the brokers in the given regions (none means all), minus those
// whose ID or name is in excluded or whose category is in excludedCategories
// (for example requires-id, for brokers that demand an ID document first).
func (db *BrokerDatabase) Filter(regions, excluded, excludedCategories []string) []Broker {
	regionSet := Set(regions)
	var out []Broker
	for _, b := range db.Select(nil, nil, nil, excluded, excludedCategories) {
		if b.InRegions(regionSet) {
			out = append(out, b)
		}
	}
	return out
}

// Select narrows the list by ID, exact region and category (an empty filter
// matches everything) and removes the excluded IDs, names and categories.
func (db *BrokerDatabase) Select(ids, regions, categories, excluded, excludedCategories []string) []Broker {
	want := map[string]map[string]bool{"id": Set(ids), "region": Set(regions), "category": Set(categories)}
	skipNames, skipCategories := Set(excluded), Set(excludedCategories)

	var out []Broker
	for _, b := range db.Brokers {
		id, name, region, category := strings.ToLower(b.ID), strings.ToLower(b.Name), strings.ToLower(b.Region), strings.ToLower(b.Category)
		switch {
		case skipNames[id], skipNames[name], skipCategories[category]:
		case len(want["id"]) > 0 && !want["id"][id]:
		case len(want["category"]) > 0 && !want["category"][category]:
		case len(want["region"]) > 0 && !want["region"][region]:
		default:
			out = append(out, b)
		}
	}
	return out
}

// UnknownIDs lists, sorted, the IDs that no broker has.
func (db *BrokerDatabase) UnknownIDs(ids []string) []string {
	var unknown []string
	for id := range Set(ids) {
		if db.FindByID(id) == nil {
			unknown = append(unknown, id)
		}
	}
	slices.Sort(unknown)
	return unknown
}
