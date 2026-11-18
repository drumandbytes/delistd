package history

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/datadir"
	_ "modernc.org/sqlite"
)

// Store is the history database: requests sent, replies received, tasks for
// a person, and the bookkeeping for the agent server.
type Store struct {
	db *sql.DB
}

// NewStore opens (and migrates) the database at dbPath, creating it with
// owner-only permissions.
func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}
	// The file holds personal data. sql.Open is lazy, so create it (or
	// tighten an existing one) before SQLite first touches it; SQLite gives
	// the -wal and -shm files the main file's mode.
	f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to create history database: %w", err)
	}
	_ = f.Close()
	if err := os.Chmod(dbPath, 0o600); err != nil {
		return nil, fmt.Errorf("failed to restrict history database permissions: %w", err)
	}

	db, err := sql.Open("sqlite", dsnFor(dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// dsnFor builds the connection string. WAL lets the web UI read while a send
// or an inbox scan writes; busy_timeout makes a writer wait for the lock
// instead of failing with SQLITE_BUSY; synchronous(NORMAL) is safe under WAL
// and saves an fsync per insert. The path goes in as an escaped file: URI so
// a '?' or '#' in it is not taken for the start of the query, and SQLite
// drops the leading '/' before a Windows drive letter. Transactions begin
// IMMEDIATE: every one here writes, and a deferred one that reads first and
// writes later fails at once with SQLITE_BUSY when another process committed
// in between, where busy_timeout would have waited.
func dsnFor(dbPath string) string {
	p := filepath.ToSlash(dbPath)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{
		Scheme:   "file",
		OmitHost: true,
		Path:     p,
		RawQuery: "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_txlock=immediate",
	}).String()
}

func (s *Store) Close() error { return s.db.Close() }

// DefaultDBPath is history.db in the data directory.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "history.db"
	}
	return filepath.Join(datadir.Of(home), "history.db")
}

// DBPathFor puts history.db next to configPath, so an alternate --config gets
// its own history; DefaultDBPath when configPath is empty.
func DBPathFor(configPath string) string {
	if configPath == "" {
		return DefaultDBPath()
	}
	return filepath.Join(filepath.Dir(configPath), "history.db")
}

// normalizeProfileID maps "" to DefaultProfileID so rows stay findable.
func normalizeProfileID(id string) string {
	if id == "" {
		return DefaultProfileID
	}
	return id
}

// Timestamps are stored as text, so they only order correctly when they all
// carry the same UTC offset: "10:00 +0300" sorts after "09:30 +0000" although
// it is earlier. Every time handed to SQLite therefore goes through utcArgs,
// and migrateTimestamps converted the rows written before this.
func utcArgs(args []any) []any {
	var out []any
	for i, a := range args {
		if t, ok := a.(time.Time); ok {
			if out == nil {
				out = append([]any(nil), args...)
			}
			out[i] = t.UTC()
		}
	}
	if out == nil {
		return args
	}
	return out
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(query, utcArgs(args)...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(query, utcArgs(args)...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(query, utcArgs(args)...)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseSQLiteTime reads a DATETIME that came back as text, as aggregates like
// MAX() return it. modernc.org/sqlite writes time.Time's String(), whose
// " m=..." monotonic suffix time.Parse rejects, so that is cut off first.
func parseSQLiteTime(s string) (time.Time, error) {
	if i := strings.Index(s, " m="); i != -1 {
		s = s[:i]
	}
	var err error
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700 -0700",
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	} {
		var t time.Time
		if t, err = time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}

// parseFlexibleTimeString is parseSQLiteTime for a nullable column; NULL and
// text that fits no known layout both give the zero time.
func parseFlexibleTimeString(s sql.NullString) time.Time {
	if !s.Valid {
		return time.Time{}
	}
	t, _ := parseSQLiteTime(s.String)
	return t
}

type scanner interface{ Scan(...any) error }
