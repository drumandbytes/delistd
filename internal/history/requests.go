package history

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"time"
)

// Add records a removal request and sets record.ID.
func (s *Store) Add(record *Record) error {
	record.ProfileID = normalizeProfileID(record.ProfileID)
	if record.SentMethod == "" {
		record.SentMethod = "smtp"
	}
	result, err := s.exec(`
		INSERT INTO removal_requests (profile_id, broker_id, broker_name, email, template, status, message_id, error, sent_at, created_at, sent_method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ProfileID, record.BrokerID, record.BrokerName, record.Email, record.Template, record.Status,
		record.MessageID, record.Error, record.SentAt, time.Now(), record.SentMethod)
	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}
	if record.ID, err = result.LastInsertId(); err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	return nil
}

// scanRecord reads the columns of the short request listing: no pipeline
// status or actor.
func scanRecord(row scanner) (*Record, error) {
	var r Record
	var sentAt, createdAt sql.NullTime
	var messageID, errText, method sql.NullString
	if err := row.Scan(&r.ID, &r.ProfileID, &r.BrokerID, &r.BrokerName, &r.Email, &r.Template,
		&r.Status, &messageID, &errText, &sentAt, &createdAt, &method); err != nil {
		return nil, err
	}
	r.MessageID, r.Error, r.SentMethod = messageID.String, errText.String, method.String
	r.SentAt, r.CreatedAt = sentAt.Time, createdAt.Time
	return &r, nil
}

// GetRecentRequests returns the newest requests of a profile, at most limit.
func (s *Store) GetRecentRequests(profileID string, limit int) ([]Record, error) {
	rows, err := s.query(`
		SELECT id, profile_id, broker_id, broker_name, email, template, status, message_id, error, sent_at, created_at, sent_method
		FROM removal_requests WHERE profile_id = ? ORDER BY sent_at DESC, id DESC LIMIT ?`, normalizeProfileID(profileID), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

const requestCols = `id, profile_id, broker_id, broker_name, email, template, status,
	message_id, error, sent_at, created_at, pipeline_status, sent_method, pipeline_actor`

func scanFullRecord(row scanner) (Record, error) {
	var r Record
	var sentAt, createdAt sql.NullTime
	var messageID, errText, stage, method, actor sql.NullString
	if err := row.Scan(&r.ID, &r.ProfileID, &r.BrokerID, &r.BrokerName, &r.Email, &r.Template,
		&r.Status, &messageID, &errText, &sentAt, &createdAt, &stage, &method, &actor); err != nil {
		return r, err
	}
	r.MessageID, r.Error, r.SentMethod, r.PipelineActor = messageID.String, errText.String, method.String, actor.String
	r.SentAt, r.CreatedAt, r.PipelineStatus = sentAt.Time, createdAt.Time, PipelineStatus(stage.String)
	return r, nil
}

func (s *Store) queryFullRecords(query string, args ...any) ([]Record, error) {
	rows, err := s.query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Record
	for rows.Next() {
		r, err := scanFullRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan request: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetAllRequests returns every request of a profile, oldest first, with the
// pipeline status (GetRecentRequests omits it). `delistd export` builds the
// evidence report from it.
func (s *Store) GetAllRequests(profileID string) ([]Record, error) {
	return s.queryFullRecords(`SELECT `+requestCols+` FROM removal_requests WHERE profile_id = ? ORDER BY sent_at ASC, id ASC`, normalizeProfileID(profileID))
}

// LatestRequests returns each broker's most recent request, newest first.
// Earlier requests to the same broker do not count towards stage totals or the
// drill-down lists, which is also how the stage updates pick their row.
func (s *Store) LatestRequests(profileID string) ([]Record, error) {
	// one index seek per broker for its newest request, not a scan of all of them
	profile := normalizeProfileID(profileID)
	out, err := s.queryFullRecords(`
		SELECT `+requestCols+` FROM removal_requests WHERE id IN (
			SELECT (SELECT id FROM removal_requests r WHERE r.profile_id = ? AND r.broker_id = b.broker_id
				ORDER BY r.sent_at DESC, r.id DESC LIMIT 1)
			FROM (SELECT DISTINCT broker_id FROM removal_requests WHERE profile_id = ?) b)
		ORDER BY sent_at DESC, id DESC`, profile, profile)
	if err != nil {
		return nil, fmt.Errorf("latest pipeline requests: %w", err)
	}
	for i := range out {
		if out[i].PipelineStatus == "" {
			out[i].PipelineStatus = PipelineEmailSent
		}
	}
	return out, nil
}

// GetStats counts a profile's requests: all, sent and failed.
func (s *Store) GetStats(profileID string) (total, sent, failed int, err error) {
	// SUM over no rows is NULL, not 0, which is what a new install has
	var sentSum, failedSum sql.NullInt64
	err = s.queryRow(`SELECT COUNT(*), SUM(CASE WHEN status='sent' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) FROM removal_requests WHERE profile_id = ?`,
		normalizeProfileID(profileID)).Scan(&total, &sentSum, &failedSum)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get stats: %w", err)
	}
	return total, int(sentSum.Int64), int(failedSum.Int64), nil
}

// GetMonthlyStats counts sent and failed requests since the start of this
// month in the local time zone.
func (s *Store) GetMonthlyStats(profileID string) (sent, failed int, err error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var sentSum, failedSum sql.NullInt64
	err = s.queryRow(`SELECT SUM(CASE WHEN status='sent' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) FROM removal_requests WHERE profile_id = ? AND sent_at >= ?`,
		normalizeProfileID(profileID), start).Scan(&sentSum, &failedSum)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get monthly stats: %w", err)
	}
	return int(sentSum.Int64), int(failedSum.Int64), nil
}

// CountSentSince counts successful sends since a time; the rolling daily cap
// (a provider's per-day sending limit) is checked against it.
func (s *Store) CountSentSince(profileID string, since time.Time) (int, error) {
	var n int
	err := s.queryRow(`SELECT COUNT(*) FROM removal_requests WHERE profile_id = ? AND status = ? AND sent_at >= ?`,
		normalizeProfileID(profileID), string(StatusSent), since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("failed to count recent sends: %w", err)
	}
	return n, nil
}

// LastSuccessfulSendTimes returns each broker's latest successful send.
func (s *Store) LastSuccessfulSendTimes(profileID string) (map[string]time.Time, error) {
	rows, err := s.query(`SELECT broker_id, MAX(sent_at) FROM removal_requests WHERE profile_id = ? AND status = ? GROUP BY broker_id`,
		normalizeProfileID(profileID), string(StatusSent))
	if err != nil {
		return nil, fmt.Errorf("failed to query last send times: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]time.Time{}
	for rows.Next() {
		var id string
		var text sql.NullString
		if err := rows.Scan(&id, &text); err != nil {
			return nil, fmt.Errorf("failed to scan last send time: %w", err)
		}
		if !text.Valid {
			continue
		}
		t, err := parseSQLiteTime(text.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last send time %q for broker %q: %w", text.String, id, err)
		}
		out[id] = t
	}
	return out, rows.Err()
}

// MarkFailed turns a broker's latest "sent" request into "failed" with a note.
// SMTP accepting a message is not delivery, and without inbox monitoring
// nothing else corrects a later bounce; this stops the resend cooldown from
// counting the broker as contacted. It returns the rows changed (0: nothing to
// fix).
func (s *Store) MarkFailed(profileID, brokerID, note string) (int64, error) {
	result, err := s.exec(`UPDATE removal_requests SET status = ?, error = ? WHERE id = (
			SELECT id FROM removal_requests WHERE profile_id = ? AND broker_id = ? AND status = ?
			ORDER BY sent_at DESC, id DESC LIMIT 1)`,
		string(StatusFailed), note, normalizeProfileID(profileID), brokerID, string(StatusSent))
	if err != nil {
		return 0, fmt.Errorf("failed to mark broker as failed: %w", err)
	}
	return result.RowsAffected()
}

// ResolveProfileForBroker names the profile that emailed brokerID most
// recently, so a reply in a shared inbox goes to the right profile and not
// merely the active one. It is the default profile when none did.
func (s *Store) ResolveProfileForBroker(brokerID string) (string, error) {
	var id sql.NullString
	err := s.queryRow(`SELECT profile_id FROM removal_requests WHERE broker_id = ? ORDER BY sent_at DESC, id DESC LIMIT 1`, brokerID).Scan(&id)
	if err == sql.ErrNoRows {
		return DefaultProfileID, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to resolve profile for broker %q: %w", brokerID, err)
	}
	return normalizeProfileID(id.String), nil
}

// ResendCooldown is long enough that resuming a backlog across the daily cap
// skips yesterday's brokers, and short enough for the monthly re-run.
const ResendCooldown = 25 * 24 * time.Hour

// SortBySendPriority puts never-sent items first, then the oldest last send,
// and keeps the given order among ties. Senders cut the list at the daily cap
// afterwards, so runs a cooldown or more apart still reach the tail of the
// list instead of re-sending its head every time.
func SortBySendPriority[T any](items []T, lastSent func(T) time.Time) {
	sort.SliceStable(items, func(i, j int) bool { return lastSent(items[i]).Before(lastSent(items[j])) })
}

const latestStatus = `(SELECT status FROM removal_requests r2 WHERE r2.profile_id = r.profile_id AND r2.broker_id = r.broker_id ORDER BY sent_at DESC, id DESC LIMIT 1)`

// parseLastSent reads a MAX(sent_at) text. A value that does not parse is
// logged and skipped: one bad row should not blank every broker's status.
func parseLastSent(text sql.NullString, brokerID string) time.Time {
	if !text.Valid {
		return time.Time{}
	}
	t, err := parseSQLiteTime(text.String)
	if err != nil {
		log.Printf("Warning: failed to parse last send time %q for broker %q: %v", text.String, brokerID, err)
	}
	return t
}

// GetAllBrokerStatuses summarises every broker a profile has contacted.
func (s *Store) GetAllBrokerStatuses(profileID string) (map[string]BrokerStatus, error) {
	rows, err := s.query(`SELECT broker_id, MAX(sent_at), `+latestStatus+`, COUNT(*)
		FROM removal_requests r WHERE profile_id = ? GROUP BY broker_id`, normalizeProfileID(profileID))
	if err != nil {
		return nil, fmt.Errorf("failed to query broker statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]BrokerStatus{}
	for rows.Next() {
		var bs BrokerStatus
		var last sql.NullString
		var status string
		if err := rows.Scan(&bs.BrokerID, &last, &status, &bs.TotalSent); err != nil {
			return nil, fmt.Errorf("failed to scan broker status: %w", err)
		}
		bs.LastSent, bs.Status = parseLastSent(last, bs.BrokerID), Status(status)
		out[bs.BrokerID] = bs
	}
	return out, rows.Err()
}

// GetBrokerStatus is GetAllBrokerStatuses for one broker, so the web UI can
// refresh a single row while a send is running.
func (s *Store) GetBrokerStatus(profileID, brokerID string) (BrokerStatus, error) {
	bs := BrokerStatus{BrokerID: brokerID}
	var last, status sql.NullString
	err := s.queryRow(`SELECT MAX(sent_at), `+latestStatus+`, COUNT(*) FROM removal_requests r WHERE profile_id = ? AND broker_id = ?`,
		normalizeProfileID(profileID), brokerID).Scan(&last, &status, &bs.TotalSent)
	if err != nil {
		return bs, fmt.Errorf("failed to query broker status: %w", err)
	}
	bs.LastSent = parseLastSent(last, brokerID)
	if status.Valid {
		bs.Status = Status(status.String)
	}
	return bs, nil
}

// DeleteByStatus deletes a profile's requests with one status.
func (s *Store) DeleteByStatus(profileID string, status Status) (int64, error) {
	result, err := s.exec(`DELETE FROM removal_requests WHERE profile_id = ? AND status = ?`, normalizeProfileID(profileID), string(status))
	if err != nil {
		return 0, fmt.Errorf("failed to delete records: %w", err)
	}
	return result.RowsAffected()
}

// DeleteAllHistory deletes a profile's requests (Settings, Danger Zone);
// replies are kept, see ClearBrokerResponses.
func (s *Store) DeleteAllHistory(profileID string) (int64, error) {
	result, err := s.exec(`DELETE FROM removal_requests WHERE profile_id = ?`, normalizeProfileID(profileID))
	if err != nil {
		return 0, fmt.Errorf("failed to delete history: %w", err)
	}
	return result.RowsAffected()
}

// GetPipelineStats counts each broker's latest request by pipeline stage.
func (s *Store) GetPipelineStats(profileID string) (map[PipelineStatus]int, error) {
	latest, err := s.LatestRequests(profileID)
	if err != nil {
		return nil, err
	}
	stats := map[PipelineStatus]int{}
	for _, r := range latest {
		stats[r.PipelineStatus]++
	}
	return stats, nil
}
