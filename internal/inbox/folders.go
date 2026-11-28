package inbox

import (
	"fmt"
	"log"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-uidplus"
)

// EnsureFolderExists creates the folder unless one by that name (in any case)
// is already there.
func (m *Monitor) EnsureFolderExists(name string) error {
	if m.client == nil {
		return errNotConnected
	}
	listing := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() { done <- m.client.List("", "*", listing) }()

	exists := false
	for box := range listing {
		exists = exists || strings.EqualFold(box.Name, name)
	}
	if err := <-done; err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}
	if exists {
		log.Printf("Folder '%s' already exists", name)
		return nil
	}
	if err := m.client.Create(name); err != nil {
		return fmt.Errorf("failed to create folder '%s': %w", name, err)
	}
	log.Printf("Created folder '%s'", name)
	return nil
}

// deletedUIDsBesides lists the messages flagged \Deleted in the selected
// mailbox other than ours. ArchiveEmails uses it to notice (it cannot prevent)
// the hazard that EXPUNGE removes every flagged message.
func (m *Monitor) deletedUIDsBesides(ours []uint32) ([]uint32, error) {
	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.DeletedFlag}
	flagged, err := m.client.UidSearch(criteria)
	if err != nil {
		return nil, err
	}
	mine := make(map[uint32]bool, len(ours))
	for _, uid := range ours {
		mine[uid] = true
	}
	var others []uint32
	for _, uid := range flagged {
		if !mine[uid] {
			others = append(others, uid)
		}
	}
	return others, nil
}

// ArchiveEmails moves the messages from the inbox folder to folder.
func (m *Monitor) ArchiveEmails(uids []uint32, folder string) error {
	if m.client == nil {
		return errNotConnected
	}
	if len(uids) == 0 {
		return nil
	}
	// make sure the right mailbox is selected
	if _, err := m.client.Select(m.config.Folder, false); err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
	}
	set := new(imap.SeqSet)
	set.AddNum(uids...)

	// MOVE (RFC 6851) is one command and atomic; fall back to copy and delete
	// where the server lacks it
	if err := m.client.UidMove(set, folder); err != nil {
		log.Printf("MOVE not supported, falling back to COPY+DELETE: %v", err)
		if err := m.copyAndDelete(set, uids, folder); err != nil {
			return err
		}
	}
	log.Printf("Archived %d emails to '%s'", len(uids), folder)
	return nil
}

// copyAndDelete is ArchiveEmails without MOVE: copy, flag \Deleted, expunge.
// With UIDPLUS (RFC 4315) the expunge is UID EXPUNGE, which removes only these
// messages. Without it a plain EXPUNGE removes every \Deleted message in the
// mailbox, so a flag another client left on other mail would lose that mail;
// that cannot be closed from here, so it warns when other messages are already
// flagged and when the number expunged is not ours.
func (m *Monitor) copyAndDelete(set *imap.SeqSet, uids []uint32, folder string) error {
	if err := m.client.UidCopy(set, folder); err != nil {
		return fmt.Errorf("failed to copy emails to '%s': %w", folder, err)
	}
	if err := m.client.UidStore(set, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return fmt.Errorf("failed to mark emails as deleted: %w", err)
	}

	scoped := uidplus.NewClient(m.client)
	if ok, _ := scoped.SupportUidPlus(); ok {
		if err := scoped.UidExpunge(set, nil); err != nil {
			return fmt.Errorf("failed to expunge archived emails: %w", err)
		}
		return nil
	}

	if others, err := m.deletedUIDsBesides(uids); err != nil {
		log.Printf("Warning: could not verify \\Deleted flag scope before expunge: %v", err)
	} else if len(others) > 0 {
		log.Printf("WARNING: %d message(s) besides the %d just archived are already flagged \\Deleted and will ALSO be removed by this expunge (UIDs: %v) - this server has no UID EXPUNGE to scope the call", len(others), len(uids), others)
	}

	gone := make(chan uint32, len(uids)+8)
	done := make(chan error, 1)
	go func() { done <- m.client.Expunge(gone) }()
	removed := 0
	for range gone {
		removed++
	}
	if err := <-done; err != nil {
		return fmt.Errorf("failed to expunge deleted emails: %w", err)
	}
	if removed != len(uids) {
		log.Printf("WARNING: expunge removed %d message(s) but this operation only intended to remove %d - other \\Deleted-flagged mail may have been removed too", removed, len(uids))
	}
	return nil
}
