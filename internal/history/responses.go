package history

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AddBrokerResponse stores a classified reply and sets resp.ID.
func (s *Store) AddBrokerResponse(resp *BrokerResponse) error {
	resp.ProfileID = normalizeProfileID(resp.ProfileID)
	now := time.Now()
	result, err := s.exec(`
		INSERT INTO broker_responses (profile_id, broker_id, broker_name, response_type, email_from, email_subject, email_body,
			form_url, confirm_url, confidence, needs_review, received_at, processed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		resp.ProfileID, resp.BrokerID, resp.BrokerName, resp.ResponseType, resp.EmailFrom, resp.EmailSubject, resp.EmailBody,
		resp.FormURL, resp.ConfirmURL, resp.Confidence, boolInt(resp.NeedsReview), resp.ReceivedAt, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert broker response: %w", err)
	}
	if resp.ID, err = result.LastInsertId(); err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	return nil
}

// AddBrokerResponseIfNew stores resp unless the same reply (profile, broker,
// subject and received time) is already there, so overlapping or scheduled
// inbox scans do not pile up duplicates. inserted is false for a reply seen
// before, and resp.ID is then the existing row's.
func (s *Store) AddBrokerResponseIfNew(resp *BrokerResponse) (inserted bool, err error) {
	var id int64
	err = s.queryRow(`SELECT id FROM broker_responses
		WHERE profile_id = ? AND broker_id = ? AND email_subject = ? AND received_at = ? LIMIT 1`,
		normalizeProfileID(resp.ProfileID), resp.BrokerID, resp.EmailSubject, resp.ReceivedAt).Scan(&id)
	switch {
	case err == nil:
		resp.ID = id
		return false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return false, fmt.Errorf("failed to check for existing broker response: %w", err)
	}
	return true, s.AddBrokerResponse(resp)
}

// Listing pages do not show bodies and can hold hundreds of rows, so they use
// the variant of the column list that leaves the body out.
const (
	brokerResponseCols = `id, profile_id, broker_id, broker_name, response_type, email_from, email_subject, email_body,
		form_url, confirm_url, confidence, needs_review, received_at, processed_at, created_at`
	brokerResponseColsNoBody = `id, profile_id, broker_id, broker_name, response_type, email_from, email_subject, '' AS email_body,
		form_url, confirm_url, confidence, needs_review, received_at, processed_at, created_at`
)

func scanBrokerResponse(row scanner) (*BrokerResponse, error) {
	var r BrokerResponse
	var review int
	var received, processed, created, body, formURL, confirmURL sql.NullString
	if err := row.Scan(&r.ID, &r.ProfileID, &r.BrokerID, &r.BrokerName, &r.ResponseType, &r.EmailFrom, &r.EmailSubject, &body,
		&formURL, &confirmURL, &r.Confidence, &review, &received, &processed, &created); err != nil {
		return nil, err
	}
	r.EmailBody, r.FormURL, r.ConfirmURL = body.String, formURL.String, confirmURL.String
	r.NeedsReview = review == 1
	r.ReceivedAt, r.ProcessedAt, r.CreatedAt = parseFlexibleTimeString(received), parseFlexibleTimeString(processed), parseFlexibleTimeString(created)
	return &r, nil
}

func (s *Store) queryBrokerResponses(query string, args ...any) ([]BrokerResponse, error) {
	rows, err := s.query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query broker responses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []BrokerResponse
	for rows.Next() {
		r, err := scanBrokerResponse(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan broker response: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// oneBrokerResponse runs a query for a single reply: nil, nil when there is none.
func (s *Store) oneBrokerResponse(what, query string, args ...any) (*BrokerResponse, error) {
	r, err := scanBrokerResponse(s.queryRow(query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to %s: %w", what, err)
	}
	return r, nil
}

// GetBrokerResponseByID returns the reply, or nil when it does not exist or
// belongs to another profile.
func (s *Store) GetBrokerResponseByID(id int64, profileID string) (*BrokerResponse, error) {
	return s.oneBrokerResponse("get broker response",
		`SELECT `+brokerResponseCols+` FROM broker_responses WHERE id = ? AND profile_id = ?`, id, normalizeProfileID(profileID))
}

func (s *Store) FindBrokerResponseBySubject(profileID, brokerID, subject string) (*BrokerResponse, error) {
	return s.oneBrokerResponse("find broker response",
		`SELECT `+brokerResponseCols+` FROM broker_responses WHERE profile_id = ? AND broker_id = ? AND email_subject = ? LIMIT 1`,
		normalizeProfileID(profileID), brokerID, subject)
}

// UpdateBrokerResponseClassification changes a reply's verdict. A reply that
// belongs to another profile is left alone.
func (s *Store) UpdateBrokerResponseClassification(id int64, profileID string, responseType string, formURL, confirmURL string, confidence float64, needsReview bool) error {
	_, err := s.exec(`UPDATE broker_responses SET response_type = ?, form_url = ?, confirm_url = ?,
		confidence = ?, needs_review = ?, processed_at = ? WHERE id = ? AND profile_id = ?`,
		responseType, formURL, confirmURL, confidence, boolInt(needsReview), time.Now(), id, normalizeProfileID(profileID))
	if err != nil {
		return fmt.Errorf("failed to update broker response: %w", err)
	}
	return nil
}

// UpdateBrokerResponseBody replaces a stored body; another profile's reply is
// left alone.
func (s *Store) UpdateBrokerResponseBody(id int64, profileID string, body string) error {
	if _, err := s.exec(`UPDATE broker_responses SET email_body = ? WHERE id = ? AND profile_id = ?`, body, id, normalizeProfileID(profileID)); err != nil {
		return fmt.Errorf("failed to update broker response body: %w", err)
	}
	return nil
}

// MarkBrokerResponseReviewed clears the review flag; sql.ErrNoRows when no
// such reply exists for the profile.
func (s *Store) MarkBrokerResponseReviewed(id int64, profileID string) error {
	result, err := s.exec(`UPDATE broker_responses SET needs_review = 0 WHERE id = ? AND profile_id = ?`, id, normalizeProfileID(profileID))
	if err != nil {
		return fmt.Errorf("failed to mark broker response reviewed: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check broker response update: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ClearBrokerResponses deletes every reply of every profile, before a full re-scan.
func (s *Store) ClearBrokerResponses() error {
	if _, err := s.exec("DELETE FROM broker_responses"); err != nil {
		return fmt.Errorf("failed to clear broker responses: %w", err)
	}
	return nil
}

// GetAllBrokerResponses returns every reply of every profile, for
// reclassification: a full re-scan processes the whole shared inbox no matter
// which profile the caller has open.
func (s *Store) GetAllBrokerResponses() ([]BrokerResponse, error) {
	return s.queryBrokerResponses(`SELECT ` + brokerResponseCols + ` FROM broker_responses ORDER BY created_at DESC, id DESC`)
}

// GetBrokerResponsesForExport returns a profile's replies with bodies, oldest
// first, for `delistd export`.
func (s *Store) GetBrokerResponsesForExport(profileID string) ([]BrokerResponse, error) {
	return s.queryBrokerResponses(`SELECT `+brokerResponseCols+` FROM broker_responses WHERE profile_id = ? ORDER BY received_at ASC, id ASC`, normalizeProfileID(profileID))
}

// GetBrokerResponses lists a profile's newest replies, without bodies,
// optionally of one type or only those needing review.
func (s *Store) GetBrokerResponses(profileID, responseType string, needsReview bool, limit int) ([]BrokerResponse, error) {
	where, args := "profile_id = ?", []any{normalizeProfileID(profileID)}
	if responseType != "" {
		where += " AND response_type = ?"
		args = append(args, responseType)
	}
	if needsReview {
		where += " AND needs_review = 1"
	}
	return s.queryBrokerResponses(`SELECT `+brokerResponseColsNoBody+` FROM broker_responses WHERE `+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, append(args, limit)...)
}

// GetResponseStats counts a profile's replies by type.
func (s *Store) GetResponseStats(profileID string) (map[string]int, error) {
	rows, err := s.query(`SELECT response_type, COUNT(*) FROM broker_responses WHERE profile_id = ? GROUP BY response_type`, normalizeProfileID(profileID))
	if err != nil {
		return nil, fmt.Errorf("failed to query response stats: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return nil, fmt.Errorf("failed to scan response stat: %w", err)
		}
		out[kind] = n
	}
	return out, rows.Err()
}
