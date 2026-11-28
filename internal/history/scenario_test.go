package history

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var updateScenario = flag.Bool("update-scenario", false, "rewrite testdata/store_scenario.json.gz from the current behaviour")

type logEntry struct {
	Call string
	Out  any
	Err  string `json:",omitempty"`
}

// scenario drives every public read and write of the store through a year of
// made-up use, two profiles and awkward inputs, logging what each call
// returned. The log is the store's behaviour in a form two versions of the
// code can be compared on.
func scenario(t *testing.T, s *Store) []logEntry {
	t.Helper()
	var log []logEntry
	rec := func(call string, out any, err error) {
		e := logEntry{Call: call, Out: out}
		if err != nil {
			e.Err = err.Error()
		}
		log = append(log, e)
	}
	day := func(n int) time.Time {
		return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Add(time.Duration(n) * 24 * time.Hour)
	}

	// requests: three profiles' worth of sends, failures, manual sends, re-sends
	n := 0
	for _, p := range []string{"default", "partner", ""} {
		for b := 0; b < 6; b++ {
			for r := 0; r < 3; r++ {
				n++
				status, errText, method := StatusSent, "", ""
				if (b+r)%5 == 4 {
					status, errText = StatusFailed, "550 no such user"
				}
				if b == 2 {
					method = "manual"
				}
				err := s.Add(&Record{ProfileID: p, BrokerID: fmt.Sprintf("b%d", b), BrokerName: fmt.Sprintf("Broker %d", b), Email: "x@b.example", Template: "gdpr",
					Status: status, Error: errText, MessageID: fmt.Sprintf("<m%d@x>", n), SentAt: day(b*3 + r*40), SentMethod: method})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for _, p := range []string{"default", "partner", "nobody", ""} {
		r, err := s.GetRecentRequests(p, 7)
		rec("GetRecentRequests "+p, r, err)
		r, err = s.GetAllRequests(p)
		rec("GetAllRequests "+p, r, err)
		a, b, c, err := s.GetStats(p)
		rec("GetStats "+p, []int{a, b, c}, err)
		a, b, err = s.GetMonthlyStats(p)
		rec("GetMonthlyStats "+p, []int{a, b}, err)
		cnt, err := s.CountSentSince(p, day(30))
		rec("CountSentSince "+p, cnt, err)
		m, err := s.LastSuccessfulSendTimes(p)
		rec("LastSuccessfulSendTimes "+p, m, err)
		st, err := s.GetAllBrokerStatuses(p)
		rec("GetAllBrokerStatuses "+p, st, err)
		for _, b := range []string{"b0", "b2", "zzz"} {
			bs, err := s.GetBrokerStatus(p, b)
			rec("GetBrokerStatus "+p+" "+b, bs, err)
		}
		ps, err := s.GetPipelineStats(p)
		rec("GetPipelineStats "+p, ps, err)
		lr, err := s.LatestRequests(p)
		rec("LatestRequests "+p, lr, err)
	}
	for _, b := range []string{"b0", "b3", "zzz"} {
		id, err := s.ResolveProfileForBroker(b)
		rec("ResolveProfileForBroker "+b, id, err)
	}

	// pipeline stages
	rec("UpdatePipelineStatus", nil, s.UpdatePipelineStatus("default", "b0", PipelineAwaitingResponse))
	rec("UpdatePipelineStatus back", nil, s.UpdatePipelineStatus("default", "b0", PipelineEmailSent))
	rec("UpdatePipelineStatusBy", nil, s.UpdatePipelineStatusBy("default", "b1", PipelineConfirmed, "mcp:x"))
	rec("OverridePipelineStatus", nil, s.OverridePipelineStatus("default", "b1", PipelineFormRequired))
	rec("UpdatePipelineStatus partner", nil, s.UpdatePipelineStatus("partner", "b2", PipelineRejected))
	lr, err := s.LatestRequests("default")
	rec("LatestRequests after stages", lr, err)
	ps, err := s.GetPipelineStats("default")
	rec("GetPipelineStats after stages", ps, err)

	// replies
	types := []string{"success", "pending", "form_required", "confirmation_required", "rejected", "no_data_held", "bounced", "unknown"}
	var respIDs []int64
	for i, ty := range types {
		for _, p := range []string{"default", "partner"} {
			r := &BrokerResponse{ProfileID: p, BrokerID: fmt.Sprintf("b%d", i%6), BrokerName: fmt.Sprintf("Broker %d", i%6), ResponseType: ty,
				EmailFrom: "privacy@b.example", EmailSubject: fmt.Sprintf("Re: ticket #%d", 100+i), EmailBody: "body of " + ty + " reply http://b.example/optout",
				FormURL: map[bool]string{true: "https://b.example/form", false: ""}[ty == "form_required"], ConfirmURL: map[bool]string{true: "https://b.example/c?t=1", false: ""}[ty == "confirmation_required"],
				Confidence: 0.5 + float64(i)/20, NeedsReview: i%3 == 0, ReceivedAt: day(60 + i), ProcessedAt: day(61 + i)}
			rec("AddBrokerResponse", nil, s.AddBrokerResponse(r))
			if p == "default" {
				respIDs = append(respIDs, r.ID)
			}
		}
	}
	dup := &BrokerResponse{ProfileID: "default", BrokerID: "b0", BrokerName: "Broker 0", ResponseType: "pending", EmailFrom: "privacy@b.example", EmailSubject: "Re: ticket #100", EmailBody: "again", ReceivedAt: day(60)}
	ins, err := s.AddBrokerResponseIfNew(dup)
	rec("AddBrokerResponseIfNew dup", ins, err)
	fresh := &BrokerResponse{ProfileID: "default", BrokerID: "b0", BrokerName: "Broker 0", ResponseType: "pending", EmailFrom: "privacy@b.example", EmailSubject: "A brand new subject", EmailBody: "new", ReceivedAt: day(70)}
	ins, err = s.AddBrokerResponseIfNew(fresh)
	rec("AddBrokerResponseIfNew fresh", ins, err)
	for _, id := range append(respIDs, 9999) {
		r, err := s.GetBrokerResponseByID(id, "default")
		rec(fmt.Sprintf("GetBrokerResponseByID %d", id), r, err)
	}
	r, err := s.GetBrokerResponseByID(respIDs[0], "partner")
	rec("GetBrokerResponseByID wrong profile", r, err)
	r, err = s.FindBrokerResponseBySubject("default", "b0", "Re: ticket #100")
	rec("FindBrokerResponseBySubject", r, err)
	r, err = s.FindBrokerResponseBySubject("default", "b0", "nope")
	rec("FindBrokerResponseBySubject none", r, err)
	rec("UpdateBrokerResponseClassification", nil, s.UpdateBrokerResponseClassification(respIDs[1], "default", "rejected", "", "", 0.91, false))
	rec("UpdateBrokerResponseBody", nil, s.UpdateBrokerResponseBody(respIDs[1], "default", "rewritten body"))
	rec("MarkBrokerResponseReviewed", nil, s.MarkBrokerResponseReviewed(respIDs[0], "default"))
	rec("MarkBrokerResponseReviewed wrong profile", nil, s.MarkBrokerResponseReviewed(respIDs[2], "nobody"))
	for _, p := range []string{"default", "partner", "nobody"} {
		for _, q := range []struct {
			ty  string
			rev bool
			lim int
		}{{"", false, 0}, {"", false, 3}, {"unknown", false, 10}, {"", true, 10}, {"success", true, 10}, {"nope", false, 10}} {
			rs, err := s.GetBrokerResponses(p, q.ty, q.rev, q.lim)
			rec(fmt.Sprintf("GetBrokerResponses %s %+v", p, q), rs, err)
		}
		fs, err := s.GetBrokerResponsesForExport(p)
		rec("GetBrokerResponsesForExport "+p, fs, err)
		rs, err := s.GetResponseStats(p)
		rec("GetResponseStats "+p, rs, err)
		for _, q := range []ResponseQuery{{}, {Type: "pending"}, {NeedsReview: true}, {Search: "ticket"}, {Search: "NEW"}, {Search: "zzz"}, {Limit: 3}, {Limit: 3, Offset: 3}, {Type: "unknown", NeedsReview: true, Search: "unknown"}} {
			rows, total, err := s.QueryBrokerResponses(p, q)
			rec(fmt.Sprintf("QueryBrokerResponses %s %+v", p, q), []any{rows, total}, err)
		}
		ids, err := s.MatchingResponseIDs(p, "ticket #10")
		rec("MatchingResponseIDs "+p, ids, err)
	}
	all, err := s.GetAllBrokerResponses()
	rec("GetAllBrokerResponses", all, err)

	// tasks and forms
	var taskIDs []int64
	for i, tt := range []struct {
		typ    TaskType
		status string
	}{{TaskCaptcha, "pending"}, {TaskManualForm, "pending"}, {TaskReview, "pending"}, {TaskManualForm, "completed"}, {TaskCaptcha, "skipped"}} {
		task := &PendingTask{ProfileID: "default", BrokerID: fmt.Sprintf("b%d", i), BrokerName: fmt.Sprintf("Broker %d", i), TaskType: tt.typ, FormURL: "https://b.example/f", Notes: "n", Status: tt.status}
		rec("AddPendingTask", nil, s.AddPendingTask(task))
		taskIDs = append(taskIDs, task.ID)
	}
	rec("AddPendingTask partner", nil, s.AddPendingTask(&PendingTask{ProfileID: "partner", BrokerID: "b0", BrokerName: "Broker 0", TaskType: TaskCaptcha, Status: "pending"}))
	for _, p := range []string{"default", "partner"} {
		for _, f := range []struct {
			tt TaskType
			st string
		}{{"", ""}, {TaskCaptcha, ""}, {"", "pending"}, {"", "completed"}, {TaskManualForm, "pending"}} {
			ts, err := s.GetPendingTasks(p, f.tt, f.st)
			rec(fmt.Sprintf("GetPendingTasks %s %v %q", p, f.tt, f.st), ts, err)
		}
		a, b, c, err := s.GetPendingTaskStats(p)
		rec("GetPendingTaskStats "+p, []int{a, b, c}, err)
		fw, err := s.GetFormsWithStatus(p)
		rec("GetFormsWithStatus "+p, fw, err)
		x1, x2, x3, x4, x5, err := s.GetFormStats(p)
		rec("GetFormStats "+p, []int{x1, x2, x3, x4, x5}, err)
		ai, err := s.GetActionItems(p)
		rec("GetActionItems "+p, ai, err)
	}
	for _, id := range append(taskIDs, 9999) {
		tk, err := s.GetPendingTaskByID(id, "default")
		rec(fmt.Sprintf("GetPendingTaskByID %d", id), tk, err)
	}
	rec("MarkTaskOpened", nil, s.MarkTaskOpened(taskIDs[0], "default"))
	rec("CompletePendingTask", nil, s.CompletePendingTask(taskIDs[1], "default", "completed"))
	rec("CompletePendingTask wrong profile", nil, s.CompletePendingTask(taskIDs[2], "partner", "completed"))
	ts, err := s.GetPendingTasks("default", "", "")
	rec("GetPendingTasks after", ts, err)

	// automation, approvals
	rec("ClaimAutomationAction", nil, s.ClaimAutomationAction("default", "k1", "subj"))
	rec("ClaimAutomationAction again", nil, s.ClaimAutomationAction("default", "k1", "subj"))
	rec("FinishAutomationAction", nil, s.FinishAutomationAction("default", "k1", "submitted", "ok"))
	at, err := s.GetAutomationAttempts("default", "subj")
	rec("GetAutomationAttempts", at, err)
	tk, err := s.EnsureAutomationTask("default", "b9", "Broker 9", TaskManualForm, "https://b.example/x", "mcp:y")
	rec("EnsureAutomationTask", tk, err)
	if tk != nil {
		rec("UpdateAutomationTask", nil, s.UpdateAutomationTask("default", tk.ID, "mcp:y", "unsure", "note", "", false))
	}
	rec("CreateBatchApproval", nil, s.CreateBatchApproval(BatchApproval{PlanID: "p1", ProfileID: "default", Digest: "d", Preview: "pv", ExpiresAt: time.Now().Add(time.Hour)}))
	pa, err := s.PendingBatchApprovals()
	rec("PendingBatchApprovals", pa, err)
	rec("ApproveBatch", nil, s.ApproveBatch("p1", "me"))
	rec("ConsumeBatchApproval", nil, s.ConsumeBatchApproval("p1", "d"))
	rec("ConsumeBatchApproval again", nil, s.ConsumeBatchApproval("p1", "d"))

	// destructive operations last
	nf, err := s.MarkFailed("default", "b3", "bounced back")
	rec("MarkFailed", nf, err)
	nd, err := s.DeleteByStatus("partner", StatusFailed)
	rec("DeleteByStatus", nd, err)
	r2, err := s.GetAllRequests("partner")
	rec("GetAllRequests partner after delete", r2, err)
	nd, err = s.DeleteAllHistory("nobody")
	rec("DeleteAllHistory nobody", nd, err)
	nd, err = s.DeleteAllHistory("default")
	rec("DeleteAllHistory default", nd, err)
	r2, err = s.GetAllRequests("default")
	rec("GetAllRequests default after delete", r2, err)
	rec("ClearBrokerResponses", nil, s.ClearBrokerResponses())
	all, err = s.GetAllBrokerResponses()
	rec("GetAllBrokerResponses after clear", all, err)
	return log
}

// stamps the store writes itself (created_at, expiry...) are replaced by a
// fixed word, so the log does not change with the clock
var nowStamp = regexp.MustCompile(`"` + regexp.QuoteMeta(time.Now().UTC().Format("2006-01-02")) + `T[^"]*"`)

func TestStoreMatchesScenario(t *testing.T) {
	time.Local = time.UTC
	s, err := NewStore(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	raw, err := json.MarshalIndent(scenario(t, s), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got := nowStamp.ReplaceAll(raw, []byte(`"<now>"`))

	file := filepath.Join("testdata", "store_scenario.json.gz")
	if *updateScenario {
		var zb bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&zb, 9)
		_, _ = zw.Write(got)
		_ = zw.Close()
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
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
	var want bytes.Buffer
	if _, err := want.ReadFrom(zr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		gl, wl := strings.Split(string(got), "\n"), strings.Split(want.String(), "\n")
		for i := 0; i < len(gl) && i < len(wl); i++ {
			if gl[i] != wl[i] {
				lo := max(i-6, 0)
				t.Fatalf("store behaviour changed at line %d of the scenario log\n got:  %s\n want: %s\n(context)\n%s", i+1, gl[i], wl[i], strings.Join(wl[lo:i], "\n"))
			}
		}
		t.Fatalf("scenario log length differs: got %d lines, want %d", len(gl), len(wl))
	}
}
