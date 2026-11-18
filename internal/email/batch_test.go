package email

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/history"
)

func brokersN(n int) []broker.Broker {
	var out []broker.Broker
	for i := 0; i < n; i++ {
		out = append(out, broker.Broker{ID: string(rune('a' + i)), Name: string(rune('A' + i)), Email: string(rune('a'+i)) + "@x.example"})
	}
	return out
}

func recFor(b broker.Broker, status history.Status, errText string) *history.Record {
	return &history.Record{BrokerID: b.ID, Status: status, Error: errText}
}

func TestBatchCountsRecordsAndSkips(t *testing.T) {
	list := brokersN(4)
	list[1].Email = " " // nothing to send to
	var recorded, skipped []string
	b := &Batch{
		Budget: -1,
		Attempt: func(_ context.Context, br broker.Broker) (*history.Record, error) {
			switch br.ID {
			case "c":
				return nil, errors.New("render failed")
			case "d":
				return recFor(br, history.StatusFailed, "connection refused"), nil
			}
			return recFor(br, history.StatusSent, ""), nil
		},
		Record:  func(r *history.Record) error { recorded = append(recorded, r.BrokerID); return nil },
		Skipped: func(_ int, br broker.Broker) { skipped = append(skipped, br.ID) },
	}
	res := b.Run(context.Background(), list)
	if res.Sent != 1 || res.Failed != 2 || res.Skipped != 1 || res.Stop != StopNone {
		t.Errorf("result = %+v", res)
	}
	if len(recorded) != 2 || recorded[0] != "a" || recorded[1] != "d" {
		t.Errorf("recorded %v; a render failure has no record to store", recorded)
	}
	if len(skipped) != 1 || skipped[0] != "b" {
		t.Errorf("skipped %v", skipped)
	}
}

func TestBatchStopsAfterThreeAuthFailuresInARow(t *testing.T) {
	var after []Attempt
	b := &Batch{
		Budget: -1,
		Attempt: func(_ context.Context, br broker.Broker) (*history.Record, error) {
			return recFor(br, history.StatusFailed, "535 Authentication failed"), nil
		},
		After: func(a Attempt) { after = append(after, a) },
	}
	res := b.Run(context.Background(), brokersN(6))
	if res.Stop != StopAuth || res.Failed != 3 || res.Left != 3 || len(after) != 3 {
		t.Errorf("result = %+v after=%d", res, len(after))
	}
}

func TestBatchAuthCounterResetsOnASuccess(t *testing.T) {
	n := 0
	b := &Batch{
		Budget: -1,
		Attempt: func(_ context.Context, br broker.Broker) (*history.Record, error) {
			n++
			if n%3 == 0 {
				return recFor(br, history.StatusSent, ""), nil
			}
			return recFor(br, history.StatusFailed, "auth error"), nil
		},
	}
	if res := b.Run(context.Background(), brokersN(9)); res.Stop != StopNone || res.Sent != 3 || res.Failed != 6 {
		t.Errorf("result = %+v", res)
	}
}

func TestBatchBudgetAndCancel(t *testing.T) {
	ok := func(_ context.Context, br broker.Broker) (*history.Record, error) {
		return recFor(br, history.StatusSent, ""), nil
	}
	res := (&Batch{Budget: 2, Attempt: ok}).Run(context.Background(), brokersN(5))
	if res.Stop != StopDailyLimit || res.Sent != 2 || res.Left != 3 {
		t.Errorf("budget: %+v", res)
	}
	if res := (&Batch{Budget: 0, Attempt: ok}).Run(context.Background(), brokersN(2)); res.Stop != StopDailyLimit || res.Left != 2 {
		t.Errorf("empty budget: %+v", res)
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &Batch{Budget: -1, Attempt: ok, After: func(a Attempt) {
		if a.Index == 1 {
			cancel()
		}
	}}
	if res := b.Run(ctx, brokersN(5)); res.Stop != StopCancelled || res.Sent != 2 || res.Left != 3 {
		t.Errorf("cancel: %+v", res)
	}
}

func TestBatchPacingAndTimeout(t *testing.T) {
	var gaps []time.Time
	b := &Batch{
		Budget: -1, RateLimit: 30 * time.Millisecond, SendTimeout: time.Second,
		Attempt: func(ctx context.Context, br broker.Broker) (*history.Record, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("no per-send deadline")
			}
			gaps = append(gaps, time.Now())
			return recFor(br, history.StatusSent, ""), nil
		},
	}
	b.Run(context.Background(), brokersN(3))
	if len(gaps) != 3 || gaps[2].Sub(gaps[0]) < 55*time.Millisecond {
		t.Errorf("attempts not paced: %v", gaps)
	}

	// a cancelled context cuts the pause short
	ctx, cancel := context.WithCancel(context.Background())
	slow := &Batch{Budget: -1, RateLimit: time.Hour, Attempt: func(_ context.Context, br broker.Broker) (*history.Record, error) {
		cancel()
		return recFor(br, history.StatusSent, ""), nil
	}}
	done := make(chan BatchResult)
	go func() { done <- slow.Run(ctx, brokersN(2)) }()
	select {
	case res := <-done:
		if res.Stop != StopCancelled {
			t.Errorf("result = %+v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the rate-limit pause ignored cancellation")
	}
}
