package email

import (
	"context"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/history"
	"github.com/drumandbytes/delistd/internal/template"
)

// MaxAuthFailures in a row stop a batch: the account or password is wrong and
// every further attempt would only add to the provider's lockout counter.
const MaxAuthFailures = 3

// StopReason says why a batch ended before the list did.
type StopReason int

const (
	StopNone       StopReason = iota // the whole list was handled
	StopCancelled                    // the context ended
	StopDailyLimit                   // the budget was used up
	StopAuth                         // MaxAuthFailures authentication failures in a row
)

// Batch sends removal requests to a list of brokers one by one. The CLI's
// `send` and the web UI's "Send all" job both run on it, so they cannot drift
// apart on skipping, the daily budget, the auth cutoff or the pacing.
type Batch struct {
	Sender   *SMTPSender
	Engine   *template.Engine
	Template string
	Profile  config.NamedProfile
	From     string

	// Record stores each attempt that produced a history record; nil stores nothing.
	Record func(*history.Record) error
	// RateLimit is the pause between attempts that sent or failed at the server.
	RateLimit time.Duration
	// SendTimeout bounds one attempt; zero means no bound beyond the context.
	SendTimeout time.Duration
	// Budget is how many emails may still be sent; negative means no limit.
	Budget int
	// Attempt replaces the real send (a dry run renders only); nil means SendRemoval.
	Attempt func(ctx context.Context, b broker.Broker) (*history.Record, error)

	// Before runs ahead of each attempt, Skipped for brokers with no email
	// address, After once an attempt is settled.
	Before  func(i int, b broker.Broker, sent, failed int)
	Skipped func(i int, b broker.Broker)
	After   func(Attempt)
}

// Attempt is what happened to one broker.
type Attempt struct {
	Index        int
	Broker       broker.Broker
	Record       *history.Record // nil when nothing was attempted (a render failure)
	Err          error           // the render failure, if any
	RecordErr    error           // storing the record failed
	Sent, Failed int             // running totals including this attempt
}

// BatchResult summarises a run.
type BatchResult struct {
	Sent, Failed, Skipped int
	Stop                  StopReason
	Left                  int // brokers not handled when it stopped early
}

func (b *Batch) Run(ctx context.Context, brokers []broker.Broker) BatchResult {
	var res BatchResult
	authFails := 0
	for i, br := range brokers {
		if ctx.Err() != nil {
			res.Stop, res.Left = StopCancelled, len(brokers)-i
			return res
		}
		// Nothing to send to (defunct, or "use the web form" in the notes):
		// skip rather than burn a daily slot on a guaranteed failure.
		if strings.TrimSpace(br.Email) == "" {
			res.Skipped++
			if b.Skipped != nil {
				b.Skipped(i, br)
			}
			continue
		}
		if b.Budget >= 0 && res.Sent >= b.Budget {
			res.Stop, res.Left = StopDailyLimit, len(brokers)-i
			return res
		}
		if b.Before != nil {
			b.Before(i, br, res.Sent, res.Failed)
		}

		rec, err := b.attempt(ctx, br)
		at := Attempt{Index: i, Broker: br, Record: rec, Err: err}
		if err != nil {
			res.Failed++
			at.Sent, at.Failed = res.Sent, res.Failed
			b.after(at)
			continue
		}
		if b.Record != nil {
			at.RecordErr = b.Record(rec)
		}
		if rec.Status == history.StatusSent {
			res.Sent++
			authFails = 0
		} else {
			res.Failed++
			if strings.Contains(strings.ToLower(rec.Error), "auth") {
				authFails++
			}
		}
		at.Sent, at.Failed = res.Sent, res.Failed
		b.after(at)
		if authFails >= MaxAuthFailures {
			res.Stop, res.Left = StopAuth, len(brokers)-i-1
			return res
		}
		if i < len(brokers)-1 && b.RateLimit > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(b.RateLimit):
			}
		}
	}
	return res
}

func (b *Batch) after(a Attempt) {
	if b.After != nil {
		b.After(a)
	}
}

func (b *Batch) attempt(ctx context.Context, br broker.Broker) (*history.Record, error) {
	if b.SendTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.SendTimeout)
		defer cancel()
	}
	if b.Attempt != nil {
		return b.Attempt(ctx, br)
	}
	return SendRemoval(ctx, b.Sender, b.Engine, b.Template, b.Profile, b.From, br)
}
