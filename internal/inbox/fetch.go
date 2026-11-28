package inbox

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
)

// go-imap v1.2.1 has no context support, so a command runs in a goroutine and
// the caller stops waiting when its context ends. The command itself keeps
// going on the shared connection until the server answers, the same caveat as
// WatchForNewEmails.
func (m *Monitor) uidSearchCtx(ctx context.Context, criteria *imap.SearchCriteria) ([]uint32, error) {
	type outcome struct {
		uids []uint32
		err  error
	}
	ch := make(chan outcome, 1)
	go func() {
		uids, err := m.client.UidSearch(criteria)
		ch <- outcome{uids, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case o := <-ch:
		return o.uids, o.err
	}
}

// fetchMessagesCtx fetches uids and parses the messages as they stream in.
// The channel is sized to the batch, so the fetching goroutine never blocks on
// it even when we stop reading early.
func (m *Monitor) fetchMessagesCtx(ctx context.Context, set *imap.SeqSet, items []imap.FetchItem, section *imap.BodySectionName, batch int) ([]Email, error) {
	stream := make(chan *imap.Message, batch)
	done := make(chan error, 1)
	go func() { done <- m.client.UidFetch(set, items, stream) }()

	var out []Email
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case msg, open := <-stream:
			if !open {
				if err := <-done; err != nil {
					return nil, fmt.Errorf("failed to fetch messages: %w", err)
				}
				return out, nil
			}
			e, err := m.parseMessage(msg, section)
			if err != nil {
				log.Printf("Warning: failed to parse message: %v", err)
				continue
			}
			if e != nil {
				out = append(out, *e)
			}
		}
	}
}

const (
	envelopeBatch = 500 // envelopes are a few hundred bytes each
	bodyBatch     = 50
)

// fetchMatching returns the mail of the last `days` days in folder that keep
// accepts, in two passes: the envelope of every message in the window, then
// full bodies for the ones kept only. Bodies use BODY.PEEK[], so scanning
// never marks anything read.
func (m *Monitor) fetchMatching(ctx context.Context, folder string, days int, keep func(Email) bool) ([]Email, error) {
	if m.client == nil {
		return nil, errNotConnected
	}
	mbox, err := m.client.Select(folder, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox %s: %w", folder, err)
	}
	if mbox.Messages == 0 {
		return nil, nil
	}

	since := time.Now().AddDate(0, 0, -days)
	criteria := imap.NewSearchCriteria()
	criteria.Since = since
	uids, err := m.uidSearchCtx(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails in %s: %w", folder, err)
	}
	if len(uids) == 0 {
		return nil, nil
	}

	// pass 1 does not ask for the body section, so parseMessage finds none
	section := &imap.BodySectionName{Peek: true}
	envelopes, err := m.fetchBatched(ctx, uids, envelopeBatch, []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid}, section)
	if err != nil {
		return nil, err
	}
	var wanted []uint32
	for _, e := range envelopes {
		if keep(e) {
			wanted = append(wanted, e.UID)
		}
	}
	log.Printf("%s: %d of %d emails since %s match", folder, len(wanted), len(uids), since.Format("2006-01-02"))

	return m.fetchBatched(ctx, wanted, bodyBatch, []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, section.FetchItem()}, section)
}

// fetchBatched fetches uids size at a time. A batch that fails is logged and
// skipped so one bad message does not sink the scan; a cancelled context
// stops it.
func (m *Monitor) fetchBatched(ctx context.Context, uids []uint32, size int, items []imap.FetchItem, section *imap.BodySectionName) ([]Email, error) {
	var out []Email
	for start := 0; start < len(uids); start += size {
		batch := uids[start:min(start+size, len(uids))]
		set := new(imap.SeqSet)
		set.AddNum(batch...)
		got, err := m.fetchMessagesCtx(ctx, set, items, section, len(batch))
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			log.Printf("Warning: error fetching batch: %v", err)
			continue
		}
		out = append(out, got...)
	}
	return out, nil
}

func fromBroker(e Email) bool { return e.BrokerID != "" }

// FetchBrokerEmails returns mail from known broker domains.
func (m *Monitor) FetchBrokerEmails(ctx context.Context, days int) ([]Email, error) {
	return m.fetchMatching(ctx, m.config.Folder, days, fromBroker)
}

// FetchBrokerEmailsFromFolder is FetchBrokerEmails for another folder.
func (m *Monitor) FetchBrokerEmailsFromFolder(ctx context.Context, folder string, days int) ([]Email, error) {
	return m.fetchMatching(ctx, folder, days, fromBroker)
}

// These are not the classifier's bounce senders: that list includes "noreply",
// which would pull every newsletter's body into a bounce scan.
var (
	bounceFetchSenders = []string{
		"mailer-daemon", "postmaster", "mail delivery", "mail delivery system",
		"mail delivery subsystem", "mailerdaemon", "mailsystem",
	}
	bounceFetchSubjects = []string{
		"undeliverable", "delivery failed", "delivery status notification", "returned mail",
		"mail delivery failed", "delivery failure", "message not delivered", "could not be delivered",
	}
)

// looksLikeBounce judges from the sender and subject alone, which the
// envelope already has.
func looksLikeBounce(e Email) bool {
	from, name, subject := strings.ToLower(e.From), strings.ToLower(e.FromName), strings.ToLower(e.Subject)
	for _, s := range bounceFetchSenders {
		if strings.Contains(from, s) || strings.Contains(name, s) {
			return true
		}
	}
	for _, s := range bounceFetchSubjects {
		if strings.Contains(subject, s) {
			return true
		}
	}
	return false
}

// FetchBounceEmails returns mail that looks like a delivery failure notice.
func (m *Monitor) FetchBounceEmails(ctx context.Context, days int) ([]Email, error) {
	return m.fetchMatching(ctx, m.config.Folder, days, looksLikeBounce)
}
