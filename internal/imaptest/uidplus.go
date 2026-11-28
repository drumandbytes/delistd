package imaptest

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-uidplus"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/responses"
	"github.com/emersion/go-imap/server"
)

// uidplusExt gives the server the UIDPLUS capability and a UID EXPUNGE that
// removes only the listed messages, which is what lets a client clear its own
// \Deleted mail without touching anybody else's. It also replaces plain
// EXPUNGE, as extensions may, so both go through the same code.
type uidplusExt struct{}

func (uidplusExt) Capabilities(c server.Conn) []string {
	if c.Context().State&imap.AuthenticatedState != 0 {
		return []string{uidplus.Capability}
	}
	return nil
}

func (uidplusExt) Command(name string) server.HandlerFactory {
	if name == "EXPUNGE" {
		return func() server.Handler { return &expunge{} }
	}
	return nil
}

type expunge struct{ cmd uidplus.ExpungeCommand }

func (e *expunge) Parse(fields []interface{}) error {
	if len(fields) == 0 {
		return nil // a plain EXPUNGE
	}
	return e.cmd.Parse(fields)
}

func (e *expunge) Handle(conn server.Conn) error { return e.expunge(conn) }

func (e *expunge) UidHandle(conn server.Conn) error { return e.expunge(conn) }

// expunge removes the \Deleted messages (those in the set, when there is
// one) and reports each removed sequence number, highest first.
func (e *expunge) expunge(conn server.Conn) error {
	ctx := conn.Context()
	mb, ok := ctx.Mailbox.(*memory.Mailbox)
	if !ok {
		return errors.New("no mailbox selected")
	}
	var kept []*memory.Message
	var gone []uint32
	for i, m := range mb.Messages {
		deleted := false
		for _, f := range m.Flags {
			deleted = deleted || f == imap.DeletedFlag
		}
		if deleted && (e.cmd.SeqSet == nil || e.cmd.SeqSet.Contains(m.Uid)) {
			gone = append(gone, uint32(i+1-len(gone))) // later numbers shift down as earlier ones go
			continue
		}
		kept = append(kept, m)
	}
	mb.Messages = kept
	out := make(chan uint32, len(gone))
	for _, n := range gone {
		out <- n
	}
	close(out)
	return conn.WriteResp(&responses.Expunge{SeqNums: out})
}
