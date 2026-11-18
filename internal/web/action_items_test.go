package web

import (
	"strings"
	"testing"
	"time"

	"github.com/drumandbytes/delistd/internal/history"
)

func TestTaskTypeNamesAndChips(t *testing.T) {
	cases := []struct {
		typ    history.TaskType
		filter string
		chip   string
	}{
		{history.TaskCaptcha, "captcha", "captcha"},
		{history.TaskManualForm, "form", "Manual Form"},
		{history.TaskReview, "review", "Review"},
		{history.TaskConfirm, "confirmation", "Confirmation"},
		{"other", "other", "other"},
	}
	for _, c := range cases {
		if got := taskFilterType(c.typ); got != c.filter {
			t.Errorf("taskFilterType(%q) = %q, want %q", c.typ, got, c.filter)
		}
		if got := taskChip(c.typ).Text; got != c.chip {
			t.Errorf("taskChip(%q) = %q, want %q", c.typ, got, c.chip)
		}
	}
	if taskChip("confirmation").Text != "Confirmation" {
		t.Error("the page's own 'confirmation' filter name should get the Confirmation chip")
	}
}

func TestBuildActionItemsOrdersAndFiltersRows(t *testing.T) {
	now := time.Now()
	forms := []history.FormWithStatus{
		{BrokerID: "a", BrokerName: "A", Status: "pending", FormURL: "https://a.example/form", DetectedAt: now},
		{BrokerID: "b", BrokerName: "B", Status: "filled"},
	}
	tasks := []history.PendingTask{
		{ID: 1, BrokerName: "C", TaskType: history.TaskCaptcha, Status: "pending", CreatedAt: now, Notes: "solve it"},
		{ID: 2, BrokerName: "D", TaskType: history.TaskManualForm, Status: "pending", FormURL: "https://d.example", CreatedAt: now},
	}
	reviews := []history.BrokerResponse{{ID: 7, BrokerName: "E", ReceivedAt: now, ResponseType: "rejected"}}
	done := []history.PendingTask{{ID: 3, BrokerName: "F", TaskType: history.TaskConfirm}}

	items, pending := buildActionItems(forms, tasks, reviews, done)
	if !pending {
		t.Error("pending rows exist")
	}
	var titles []string
	for _, it := range items {
		titles = append(titles, it.Title)
	}
	if got := strings.Join(titles, ","); got != "A,C,D,E,F,B" {
		t.Errorf("order = %s, want pending form, tasks, review, completed task, filled form (A,C,D,E,F,B)", got)
	}

	byTitle := map[string]actionItem{}
	for _, it := range items {
		byTitle[it.Title] = it
	}
	if a := byTitle["A"].Actions; len(a) != 3 || !a[0].External || a[1].Post != "/forms/a/complete" || a[2].Post != "/forms/a/skip" {
		t.Errorf("form actions: %+v", a)
	}
	if a := byTitle["C"].Actions[0]; a.Href != "/tasks/1/helper" {
		t.Errorf("captcha task opens the helper, got %+v", a)
	}
	if a := byTitle["D"].Actions[0]; a.Href != "https://d.example" || !a.External {
		t.Errorf("form task opens the form, got %+v", a)
	}
	if a := byTitle["E"].Actions; a[0].Href != "/pipeline/responses/7" || a[1].Post != "/api/pipeline/responses/7/reviewed" {
		t.Errorf("review actions: %+v", a)
	}
	if !byTitle["F"].Done || !byTitle["B"].Done {
		t.Error("finished rows are marked done")
	}

	if _, pending := buildActionItems(nil, nil, nil, done); pending {
		t.Error("finished rows alone are not pending")
	}
}
