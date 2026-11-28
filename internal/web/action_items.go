package web

import (
	"fmt"

	"github.com/drumandbytes/delistd/internal/history"
)

// The Action Needed page lists several kinds of work in one list. They are
// turned into the same shape here so the template draws every row the same way.

const timeFormat = "Jan 2, 2006 3:04 PM"

type chip struct{ Class, Text string }

// itemLine is one line of detail; a Link makes the text a link.
type itemLine struct {
	Class string
	Text  string
	Link  string
}

// itemAction is a button or link at the end of a row.
type itemAction struct {
	Label, Class, Icon string
	Href               string // a link when set,
	Post, Confirm      string // else a button that POSTs here after confirming
	External           bool   // opens in a new tab
}

type actionItem struct {
	ID      string // the element id, when other code needs to find the row
	Type    string // what the filter buttons match: form, captcha, confirmation, review ...
	Status  string // "completed" rows are hidden until that filter is chosen
	Title   string
	Chips   []chip
	Aside   string
	Lines   []itemLine
	Actions []itemAction
	Done    bool // a finished row shows a "Done" mark instead of actions
}

func (i actionItem) Hidden() bool { return i.Status == "completed" }

const (
	btnSmall   = "btn text-xs px-3 py-1.5"
	chipInfo   = "chip chip-info"
	chipWarn   = "chip chip-warning"
	chipDone   = "chip chip-success"
	lineMuted  = "text-xs text-muted mt-1"
	lineLink   = "text-xs text-accent mt-1 truncate"
	lineSubj   = "text-xs text-muted mt-1 truncate"
	taskKind   = "task"
	noteClass  = "text-sm text-secondary mt-2"
	stampClass = "text-xs text-muted mt-2"
)

// taskType is the name the filter buttons use for a task type.
func taskFilterType(t history.TaskType) string {
	switch t {
	case history.TaskManualForm:
		return "form"
	case history.TaskConfirm:
		return "confirmation"
	}
	return string(t)
}

func taskChip(t history.TaskType) chip {
	switch t {
	case history.TaskCaptcha:
		return chip{chipWarn, "captcha"}
	case history.TaskManualForm:
		return chip{chipInfo, "Manual Form"}
	case history.TaskReview:
		return chip{chipWarn, "Review"}
	case history.TaskConfirm, "confirmation":
		return chip{chipInfo, "Confirmation"}
	}
	return chip{"chip chip-neutral", string(t)}
}

func formItem(f history.FormWithStatus) actionItem {
	return actionItem{
		ID: "form-" + f.BrokerID, Type: "form", Status: "pending", Title: f.BrokerName,
		Chips: []chip{{chipInfo, "Form"}},
		Lines: []itemLine{
			{lineSubj, f.EmailSubject, ""},
			{lineLink, f.FormURL, f.FormURL},
			{lineMuted, "Detected " + f.DetectedAt.Format(timeFormat), ""},
		},
		Actions: []itemAction{
			{Label: "Open Form", Class: btnSmall + " btn-secondary", Icon: "external-link", Href: f.FormURL, External: true},
			{Label: "Done", Class: btnSmall + " btn-primary", Icon: "check", Post: "/forms/" + f.BrokerID + "/complete", Confirm: "Mark this form as complete?"},
			{Class: "btn btn-ghost text-xs px-2 py-1.5", Icon: "chevron-double-right", Post: "/forms/" + f.BrokerID + "/skip", Confirm: "Skip this item?"},
		},
	}
}

func taskItem(t history.PendingTask) actionItem {
	it := actionItem{
		Type: taskFilterType(t.TaskType), Status: t.Status, Title: t.BrokerName,
		Chips: []chip{taskChip(t.TaskType)}, Aside: t.ActorLabel(),
	}
	if t.NeedsReview() {
		it.Chips = append(it.Chips, chip{chipWarn, "Needs review"})
	}
	if t.FormURL != "" {
		it.Lines = append(it.Lines, itemLine{lineLink, t.FormURL, t.FormURL})
	}
	if t.Notes != "" {
		it.Lines = append(it.Lines, itemLine{noteClass, t.Notes, ""})
	}
	it.Lines = append(it.Lines, itemLine{stampClass, "Created " + t.CreatedAt.Format(timeFormat), ""})

	switch {
	case t.TaskType == history.TaskCaptcha:
		it.Actions = append(it.Actions, itemAction{Label: "Open Helper", Class: btnSmall + " btn-primary", Icon: "external-link", Href: fmt.Sprintf("/tasks/%d/helper", t.ID)})
	case t.FormURL != "":
		it.Actions = append(it.Actions, itemAction{Label: "Open", Class: btnSmall + " btn-secondary", Icon: "external-link", Href: t.FormURL, External: true})
	}
	it.Actions = append(it.Actions, itemAction{Label: "Done", Class: btnSmall + " btn-primary", Icon: "check",
		Post: fmt.Sprintf("/tasks/%d/complete", t.ID), Confirm: "Mark this task as complete?"})
	return it
}

func reviewItem(r history.BrokerResponse) actionItem {
	return actionItem{
		Type: "review", Status: "pending", Title: r.BrokerName,
		Chips: []chip{{chipWarn, "Needs Review"}},
		Lines: []itemLine{
			{lineSubj, r.EmailSubject, ""},
			{"text-xs text-secondary mt-1", "Response type: " + pipelineLabel(r.ResponseType), ""},
			{lineMuted, "Received " + r.ReceivedAt.Format(timeFormat), ""},
		},
		Actions: []itemAction{
			{Label: "Review", Class: btnSmall + " btn-secondary", Icon: "eye", Href: fmt.Sprintf("/pipeline/responses/%d", r.ID)},
			{Label: "Done", Class: btnSmall + " btn-primary", Icon: "check", Post: fmt.Sprintf("/api/pipeline/responses/%d/reviewed", r.ID), Confirm: "Mark this as reviewed?"},
		},
	}
}

func finishedItem(typ, title, what string) actionItem {
	return actionItem{Type: typ, Status: "completed", Title: title, Chips: []chip{{chipDone, "Completed"}},
		Lines: []itemLine{{lineMuted, what, ""}}, Done: true}
}

// buildActionItems lists the page's rows in display order: pending forms,
// tasks, reviews, then the finished ones that stay hidden until the Completed
// filter is chosen. pending says whether any visible row exists.
func buildActionItems(forms []history.FormWithStatus, tasks []history.PendingTask, reviews []history.BrokerResponse, completed []history.PendingTask) (items []actionItem, pending bool) {
	for _, f := range forms {
		if f.Status == "pending" {
			items = append(items, formItem(f))
		}
	}
	for _, t := range tasks {
		items = append(items, taskItem(t))
	}
	for _, r := range reviews {
		items = append(items, reviewItem(r))
	}
	pending = len(items) > 0
	for _, t := range completed {
		items = append(items, finishedItem(taskFilterType(t.TaskType), t.BrokerName, "Task completed"))
	}
	for _, f := range forms {
		if f.Status == "filled" {
			items = append(items, finishedItem("form", f.BrokerName, "Form submitted successfully"))
		}
	}
	return items, pending
}
