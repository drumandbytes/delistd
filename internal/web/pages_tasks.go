package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/drumandbytes/delistd/internal/history"
	"github.com/go-chi/chi/v5"
)

// The "forms" routes predate the unified Action Needed page; the list moved
// to /tasks and the two actions now set the pipeline stage.
func (s *Server) handleForms(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/tasks", http.StatusFound)
}

func (s *Server) handleFormComplete(w http.ResponseWriter, r *http.Request) {
	s.settleForm(w, r, history.PipelineConfirmed)
}

func (s *Server) handleFormSkip(w http.ResponseWriter, r *http.Request) {
	s.settleForm(w, r, history.PipelineRejected)
}

func (s *Server) settleForm(w http.ResponseWriter, r *http.Request, stage history.PipelineStatus) {
	store := s.store(w, "Database not available")
	if store == nil {
		return
	}
	if err := store.OverridePipelineStatus(s.activeProfile(r).ID, chi.URLParam(r, "brokerID"), stage); err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/tasks")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tasks", http.StatusFound)
}

// taskFigure is one headline number; a Filter makes it a button for that filter.
type taskFigure struct {
	Label  string
	Value  int
	Filter string
}

// taskFilter is a filter button. A negative Count shows no number.
type taskFilter struct {
	Key, Label, Icon string
	Count            int
	Pill             string
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if cfg := s.getConfig(); cfg == nil || !cfg.HasProfile() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	active := s.activeProfile(r)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}

	var (
		actions            history.ActionItems
		forms              []history.FormWithStatus
		completedList      []history.PendingTask
		completedTasks     int
		skippedTasks       int
		filled, captcha    int
		failedForms, skips int
	)
	if store := s.historyStore; store != nil {
		var err error
		if actions, err = store.GetActionItems(active.ID); err != nil {
			http.Error(w, "Unable to load action items", http.StatusInternalServerError)
			return
		}
		all, err := store.GetFormsWithStatus(active.ID)
		if err != nil {
			http.Error(w, "Unable to load forms", http.StatusInternalServerError)
			return
		}
		// pending forms come only from the shared action projection
		forms = append(forms, actions.Forms...)
		for _, f := range all {
			if f.Status != "pending" {
				forms = append(forms, f)
			}
		}
		completedList, _ = store.GetPendingTasks(active.ID, "", "completed")
		_, completedTasks, skippedTasks, _ = store.GetPendingTaskStats(active.ID)
		_, filled, captcha, failedForms, skips, _ = store.GetFormStats(active.ID)
	}

	items, hasPending := buildActionItems(forms, actions.Tasks, actions.Reviews, completedList)
	completedCount := completedTasks + filled
	s.renderWithCSRF(w, r, "tasks.html", map[string]interface{}{
		"Title":      "Action Needed",
		"HowTo":      taskHowTo,
		"Items":      items,
		"HasPending": hasPending,
		"Figures": []taskFigure{
			{"Total Action Items", actions.Total(), ""},
			{"Forms to Fill", actions.FormsToFill(), "form"},
			{"Needs Review", actions.NeedsReview(), "review"},
			{"Completed", completedCount, "completed"},
		},
		"Filters": []taskFilter{
			{Key: "all", Label: "All", Count: actions.Total(), Pill: "ml-1 text-xs px-1.5 py-0.5 rounded-full bg-elevated"},
			{Key: "form", Label: "Forms", Icon: "document-text", Count: actions.FormsToFill(), Pill: "ml-1 rounded-full " + chipInfo},
			{Key: "captcha", Label: "CAPTCHA", Icon: "shield-check", Count: captcha, Pill: "ml-1 rounded-full " + chipWarn},
			{Key: "confirmation", Label: "Confirmation", Icon: "mail", Count: -1},
			{Key: "review", Label: "Needs Review", Icon: "eye", Count: actions.NeedsReview(), Pill: "ml-1 rounded-full " + chipWarn},
			{Key: "completed", Label: "Completed", Icon: "check-circle", Count: completedCount, Pill: "ml-1 rounded-full " + chipDone},
		},
		"Tasks":              actions.Tasks,
		"CompletedTasksList": completedList,
		"Forms":              forms,
		"ReviewItems":        actions.Reviews,
		"TaskType":           r.URL.Query().Get("type"),
		"Status":             status,
		"PendingTasks":       len(actions.Tasks),
		"CompletedTasks":     completedTasks,
		"SkippedTasks":       skippedTasks,
		"PendingForms":       len(actions.Forms),
		"FilledForms":        filled,
		"CaptchaForms":       captcha,
		"FailedForms":        failedForms,
		"SkippedForms":       skips,
		"NeedsReview":        actions.NeedsReview(),
		"FormsNeedingAction": actions.FormsToFill(),
		"TotalActionItems":   actions.Total(),
	})
}

// taskFor loads the task a request is about, answering 404 itself when the id
// is not a number or no such task exists for the active profile.
func (s *Server) taskFor(w http.ResponseWriter, r *http.Request) (*history.PendingTask, int64, bool) {
	id, ok := urlID(r, "taskID")
	store := s.store(w, "Database not available")
	if store == nil {
		return nil, 0, false
	}
	var task *history.PendingTask
	if ok {
		task, _ = store.GetPendingTaskByID(id, s.activeProfile(r).ID)
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return nil, 0, false
	}
	return task, id, true
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	task, _, ok := s.taskFor(w, r)
	if !ok {
		return
	}
	note, steps := taskInstructions(*task)
	s.renderWithCSRF(w, r, "task-detail.html", map[string]interface{}{"Title": "Task Detail", "Task": task, "Note": note, "Steps": steps})
}

func (s *Server) handleTaskComplete(w http.ResponseWriter, r *http.Request) {
	limitFormBody(w, r)
	status := r.FormValue("status")
	if status == "" {
		status = "completed"
	}
	s.finishTask(w, r, status)
}

func (s *Server) handleTaskSkip(w http.ResponseWriter, r *http.Request) {
	s.finishTask(w, r, "skipped")
}

// finishTask records the outcome and sends the browser back to the helper
// page. Failures answer with a small HTML fragment, since HTMX swaps the
// response into the page.
func (s *Server) finishTask(w http.ResponseWriter, r *http.Request, status string) {
	id, ok := urlID(r, "taskID")
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	if s.historyStore == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<span class="text-error">Database not available</span>`))
		return
	}
	if err := s.historyStore.CompletePendingTask(id, s.activeProfile(r).ID, status); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `<span class="text-error">Error: %s</span>`, template.HTMLEscapeString(err.Error()))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/tasks/%d/helper", id), http.StatusFound)
}

// the order and wording of the profile fields the helper page offers to copy
var helperFields = []struct{ Key, Label string }{
	{"email", "Email"}, {"firstName", "First Name"}, {"lastName", "Last Name"}, {"phone", "Phone"},
	{"address", "Address"}, {"city", "City"}, {"state", "State"}, {"zipCode", "ZIP Code"}, {"country", "Country"},
}

func (s *Server) handleTaskHelper(w http.ResponseWriter, r *http.Request) {
	task, id, ok := s.taskFor(w, r)
	if !ok {
		return
	}
	profileID := s.activeProfile(r).ID
	// stamps opened_at the first time; read the task again to show it
	_ = s.historyStore.MarkTaskOpened(id, profileID)
	if again, _ := s.historyStore.GetPendingTaskByID(id, profileID); again != nil {
		task = again
	}

	profile := map[string]string{}
	if task.BrowserState != "" {
		_ = json.Unmarshal([]byte(task.BrowserState), &profile)
	}
	ordered := make([]map[string]string, 0, len(helperFields))
	for _, f := range helperFields {
		if v := profile[f.Key]; v != "" {
			ordered = append(ordered, map[string]string{"key": f.Label, "value": v})
		}
	}
	s.renderWithCSRF(w, r, "task-helper.html", map[string]interface{}{
		"Title":          "CAPTCHA Task: " + task.BrokerName,
		"Task":           task,
		"ProfileData":    profile,
		"OrderedProfile": ordered,
	})
}

// taskInstructions is what the detail page tells a person to do: numbered
// steps for a task still open, and a plain note when there are none to give.
func taskInstructions(t history.PendingTask) (note string, steps []string) {
	if t.Status == "completed" {
		return "This task is complete. Review the saved notes and screenshot for its outcome. A submitted request does not by itself confirm that your data was removed.", nil
	}
	switch t.TaskType {
	case history.TaskCaptcha:
		return "", []string{
			`Click "Open Form" to navigate to the data broker's opt-out form.`,
			"Your information may already be filled in. Verify it's correct.",
			"Solve the CAPTCHA challenge (reCAPTCHA, hCaptcha, etc.).",
			"Submit the form and return here to mark the task complete.",
		}
	case history.TaskManualForm:
		return "", []string{
			`Click "Open Form" to navigate to the opt-out form.`,
			"Fill in your personal information as required.",
			"Submit the form to complete your opt-out request.",
			"Return here and mark the task as complete.",
		}
	case history.TaskReview:
		return "", []string{
			"Review the broker's response in your email inbox.",
			"Determine if action is needed (form, confirmation, etc.).",
			"Take appropriate action based on the response.",
			"Mark this task complete after reviewing.",
		}
	case "confirmation":
		return "", []string{
			"Check your email for a confirmation message from the broker.",
			"Click the confirmation link in the email.",
			"Wait for the confirmation page to load.",
			"Return here and mark the task as complete.",
		}
	}
	return "Review the task details and take appropriate action.", nil
}

// taskHowTo is the short guide under the list of tasks.
var taskHowTo = []struct{ Title, Text string }{
	{"Opt-Out Forms", `Some brokers require filling out an online form. Click "Open Form", enter your details, and submit.`},
	{"CAPTCHA Forms", `Click "Solve CAPTCHA" for a helper page with copy buttons for your info. Complete the CAPTCHA, then paste your details.`},
	{"Email Confirmations", "Check your email for confirmation links from brokers. Click the link to verify your removal request."},
}
