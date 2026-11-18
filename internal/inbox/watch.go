package inbox

import (
	"context"
	"fmt"
	"log"

	"github.com/emersion/go-imap/client"
)

// WatchForNewEmails blocks, calling callback for each new broker email, until
// ctx ends or the connection fails. It waits in IMAP IDLE and fetches the last
// day's mail whenever the server reports a change.
func (m *Monitor) WatchForNewEmails(ctx context.Context, callback func(Email)) error {
	if m.client == nil {
		return errNotConnected
	}
	if _, err := m.client.Select(m.config.Folder, false); err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
	}

	// The client blocks on its Updates channel until it is read, and the
	// fetch below SELECTs again, which emits EXISTS into it; so something
	// always reads it. That goroutine lives as long as the connection.
	updates := make(chan client.Update, 16)
	newMail := make(chan struct{}, 1)
	go func() {
		for u := range updates {
			if _, ok := u.(*client.MailboxUpdate); ok {
				select {
				case newMail <- struct{}{}:
				default: // one pending wake-up is enough
				}
			}
		}
	}()
	m.client.Updates = updates

	// idle runs IDLE until stop is closed; the result arrives on idleDone
	idleDone := make(chan error, 1)
	idle := func() chan struct{} {
		stop := make(chan struct{})
		go func() { idleDone <- m.client.Idle(stop, nil) }()
		return stop
	}
	// halt ends IDLE and waits for it, so the connection is free to use
	halt := func(stop chan struct{}) {
		close(stop)
		if err := <-idleDone; err != nil {
			log.Printf("IDLE ended with: %v", err)
		}
	}
	stop := idle()
	log.Printf("Watching for new emails (press Ctrl+C to stop)...")

	for {
		select {
		case <-ctx.Done():
			halt(stop) // the caller must not use the connection while IDLE still holds it
			return ctx.Err()
		case <-newMail:
			log.Printf("New mail detected")
			halt(stop)

			emails, err := m.FetchBrokerEmails(ctx, 1)
			if err != nil {
				log.Printf("Error fetching new email: %v", err)
			}
			for _, e := range emails {
				callback(e)
			}
			select { // the fetch's own SELECT reported the mail again
			case <-newMail:
			default:
			}
			stop = idle()
		case err := <-idleDone:
			if err != nil {
				return fmt.Errorf("IDLE error: %w", err)
			}
		}
	}
}
