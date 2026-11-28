package history

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Columns added to tables that older versions created without them. They run
// before the CREATE TABLE IF NOT EXISTS statements below: on a fresh install
// the table is not there yet and the ALTER is a no-op.
var addedColumns = []string{
	`ALTER TABLE removal_requests ADD COLUMN pipeline_status TEXT DEFAULT 'email_sent'`,
	`ALTER TABLE removal_requests ADD COLUMN sent_method TEXT DEFAULT 'smtp'`, // before manual mode every send was SMTP
	`ALTER TABLE removal_requests ADD COLUMN pipeline_actor TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE pending_tasks ADD COLUMN actor TEXT NOT NULL DEFAULT 'human'`,
	`ALTER TABLE pending_tasks ADD COLUMN review_reason TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE pending_tasks ADD COLUMN opened_at DATETIME`,
	`ALTER TABLE broker_responses ADD COLUMN email_body TEXT`, // queried everywhere but missing from the first schema
	// rows from before profiles existed become the default profile's
	`ALTER TABLE removal_requests ADD COLUMN profile_id TEXT NOT NULL DEFAULT '` + DefaultProfileID + `'`,
	`ALTER TABLE broker_responses ADD COLUMN profile_id TEXT NOT NULL DEFAULT '` + DefaultProfileID + `'`,
	`ALTER TABLE pending_tasks ADD COLUMN profile_id TEXT NOT NULL DEFAULT '` + DefaultProfileID + `'`,
}

// addColumnIfMissing runs an ALTER TABLE ... ADD COLUMN, ignoring only the
// two errors that mean there is nothing to do: the column exists, or the
// table does not exist yet.
func addColumnIfMissing(db *sql.DB, alter string) error {
	_, err := db.Exec(alter)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "no such table") {
		return nil
	}
	return fmt.Errorf("migration failed: %w", err)
}

const schema = `
CREATE TABLE IF NOT EXISTS removal_requests (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id TEXT NOT NULL DEFAULT '` + DefaultProfileID + `',
	broker_id TEXT NOT NULL,
	broker_name TEXT NOT NULL,
	email TEXT NOT NULL,
	template TEXT NOT NULL,
	status TEXT NOT NULL,
	message_id TEXT,
	error TEXT,
	sent_at DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	pipeline_status TEXT DEFAULT 'email_sent',
	pipeline_actor TEXT NOT NULL DEFAULT '',
	sent_method TEXT DEFAULT 'smtp'
);
CREATE INDEX IF NOT EXISTS idx_broker_id ON removal_requests(broker_id);
CREATE INDEX IF NOT EXISTS idx_sent_at ON removal_requests(sent_at);
CREATE INDEX IF NOT EXISTS idx_status ON removal_requests(status);
CREATE INDEX IF NOT EXISTS idx_pipeline_status ON removal_requests(pipeline_status);
CREATE INDEX IF NOT EXISTS idx_profile_id ON removal_requests(profile_id);
-- every "latest request for this broker in this profile" lookup (MarkFailed,
-- the pipeline updates, the status queries) is an index scan, not a table
-- scan plus sort per broker
CREATE INDEX IF NOT EXISTS idx_broker_sent_at ON removal_requests(profile_id, broker_id, sent_at DESC);

CREATE TABLE IF NOT EXISTS broker_responses (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id TEXT NOT NULL DEFAULT '` + DefaultProfileID + `',
	broker_id TEXT NOT NULL,
	broker_name TEXT NOT NULL,
	response_type TEXT NOT NULL,
	email_from TEXT,
	email_subject TEXT,
	email_body TEXT,
	form_url TEXT,
	confirm_url TEXT,
	confidence REAL,
	needs_review INTEGER DEFAULT 0,
	received_at DATETIME,
	processed_at DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_br_broker_id ON broker_responses(broker_id);
CREATE INDEX IF NOT EXISTS idx_br_response_type ON broker_responses(response_type);
CREATE INDEX IF NOT EXISTS idx_br_needs_review ON broker_responses(needs_review);
CREATE INDEX IF NOT EXISTS idx_br_profile_id ON broker_responses(profile_id);

CREATE TABLE IF NOT EXISTS pending_tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id TEXT NOT NULL DEFAULT '` + DefaultProfileID + `',
	broker_id TEXT NOT NULL,
	broker_name TEXT NOT NULL,
	task_type TEXT NOT NULL,
	form_url TEXT,
	screenshot_path TEXT,
	browser_state TEXT,
	notes TEXT,
	status TEXT DEFAULT 'pending',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	opened_at DATETIME,
	actor TEXT NOT NULL DEFAULT 'human',
	review_reason TEXT NOT NULL DEFAULT '',
	completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_pt_broker_id ON pending_tasks(broker_id);
CREATE INDEX IF NOT EXISTS idx_pt_task_type ON pending_tasks(task_type);
CREATE INDEX IF NOT EXISTS idx_pt_status ON pending_tasks(status);
CREATE INDEX IF NOT EXISTS idx_pt_profile_id ON pending_tasks(profile_id);
`

func (s *Store) migrate() error {
	for _, alter := range addedColumns {
		if err := addColumnIfMissing(s.db, alter); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	if err := s.migrateApprovals(); err != nil {
		return err
	}
	if err := s.migrateAutomation(); err != nil {
		return err
	}
	return s.migrateTimestamps()
}

// timestampColumns are every DATETIME column a person or the clock fills in.
var timestampColumns = []struct{ table, column string }{
	{"removal_requests", "sent_at"}, {"removal_requests", "created_at"},
	{"broker_responses", "received_at"}, {"broker_responses", "processed_at"}, {"broker_responses", "created_at"},
	{"pending_tasks", "created_at"}, {"pending_tasks", "opened_at"}, {"pending_tasks", "completed_at"},
	{"batch_approvals", "created_at"}, {"batch_approvals", "expires_at"}, {"batch_approvals", "approved_at"}, {"batch_approvals", "consumed_at"},
	{"automation_attempts", "updated_at"},
}

const utcMigration = "utc_timestamps_v1"

// migrateTimestamps rewrites, once, every stored time in UTC (see utcArgs);
// the driver writes a UTC time as RFC 3339 ending in Z, so anything else is
// converted.
// Rows written by earlier versions carry the local offset of the day they were
// written, so around a clock change or a move between time zones "latest send"
// and the daily send count came out wrong. A value that cannot be parsed is
// left as it is.
func (s *Store) migrateTimestamps() error {
	// the common case, a database already converted, needs no write lock, so
	// routine opens (`delistd status` while a send is running) do not contend
	var applied int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'data_migrations'`).Scan(&applied)
	if err == nil && applied > 0 {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM data_migrations WHERE name = ?`, utcMigration).Scan(&applied)
	}
	if err != nil {
		return err
	}
	if applied > 0 {
		return nil
	}

	tx, err := s.db.Begin() // IMMEDIATE (see dsnFor): takes the write lock now and waits for it
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS data_migrations (name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL)`); err != nil {
		return err
	}
	claimed, err := tx.Exec(`INSERT INTO data_migrations (name, applied_at) VALUES (?, ?) ON CONFLICT(name) DO NOTHING`, utcMigration, time.Now().UTC())
	if err != nil {
		return err
	}
	if n, err := claimed.RowsAffected(); err != nil || n == 0 {
		return err // another process got there first
	}
	for _, tc := range timestampColumns {
		if err := rewriteAsUTC(tx, tc.table, tc.column); err != nil {
			return fmt.Errorf("convert %s.%s to UTC: %w", tc.table, tc.column, err)
		}
	}
	return tx.Commit()
}

func rewriteAsUTC(tx *sql.Tx, table, column string) error {
	rows, err := tx.Query(fmt.Sprintf(`SELECT rowid, %[1]s FROM %[2]s WHERE %[1]s IS NOT NULL AND %[1]s NOT LIKE '%%Z'`, column, table))
	if err != nil {
		return err
	}
	type fix struct {
		rowid int64
		at    time.Time
	}
	var fixes []fix
	for rows.Next() {
		var id int64
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			_ = rows.Close()
			return err
		}
		if t, err := parseSQLiteTime(text); err == nil {
			fixes = append(fixes, fix{id, t.UTC()})
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	update := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE rowid = ?`, table, column)
	for _, f := range fixes {
		if _, err := tx.Exec(update, f.at, f.rowid); err != nil {
			return err
		}
	}
	return nil
}
