package history

import (
	"database/sql"
	"fmt"
	"time"
)

// AddPendingTask creates a task for a person and sets task.ID. New tasks are
// always "pending"; the actor defaults to "human".
func (s *Store) AddPendingTask(task *PendingTask) error {
	task.ProfileID = normalizeProfileID(task.ProfileID)
	if task.Actor == "" {
		task.Actor = "human"
	}
	result, err := s.exec(`
		INSERT INTO pending_tasks (profile_id, broker_id, broker_name, task_type, form_url, screenshot_path,
			browser_state, notes, status, created_at, actor, review_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ProfileID, task.BrokerID, task.BrokerName, task.TaskType, task.FormURL, task.ScreenshotPath,
		task.BrowserState, task.Notes, "pending", time.Now(), task.Actor, task.ReviewReason)
	if err != nil {
		return fmt.Errorf("failed to insert pending task: %w", err)
	}
	if task.ID, err = result.LastInsertId(); err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	return nil
}

const taskCols = `id, profile_id, broker_id, broker_name, task_type, form_url, screenshot_path,
	browser_state, notes, status, created_at, opened_at, completed_at, actor, review_reason`

func scanTask(row scanner) (PendingTask, error) {
	var t PendingTask
	var created sql.NullTime
	var formURL, screenshot, state, notes sql.NullString
	err := row.Scan(&t.ID, &t.ProfileID, &t.BrokerID, &t.BrokerName, &t.TaskType, &formURL, &screenshot,
		&state, &notes, &t.Status, &created, &t.OpenedAt, &t.CompletedAt, &t.Actor, &t.ReviewReason)
	t.FormURL, t.ScreenshotPath, t.BrowserState, t.Notes = formURL.String, screenshot.String, state.String, notes.String
	t.CreatedAt = created.Time
	return t, err
}

// GetPendingTasks lists a profile's tasks, newest first, optionally of one
// type and/or status.
func (s *Store) GetPendingTasks(profileID string, taskType TaskType, status string) ([]PendingTask, error) {
	where, args := "profile_id = ?", []any{normalizeProfileID(profileID)}
	if taskType != "" {
		where += " AND task_type = ?"
		args = append(args, taskType)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	rows, err := s.query(`SELECT `+taskCols+` FROM pending_tasks WHERE `+where+` ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []PendingTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending task: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetPendingTaskByID returns the task, or nil, nil when it does not exist or
// belongs to another profile (the two look the same on purpose).
func (s *Store) GetPendingTaskByID(id int64, profileID string) (*PendingTask, error) {
	t, err := scanTask(s.queryRow(`SELECT `+taskCols+` FROM pending_tasks WHERE id = ? AND profile_id = ?`, id, normalizeProfileID(profileID)))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query pending task: %w", err)
	}
	return &t, nil
}

// CompletePendingTask sets a task's final status; another profile's task is
// left alone.
func (s *Store) CompletePendingTask(id int64, profileID string, status string) error {
	if _, err := s.exec(`UPDATE pending_tasks SET status = ?, completed_at = ? WHERE id = ? AND profile_id = ?`, status, time.Now(), id, normalizeProfileID(profileID)); err != nil {
		return fmt.Errorf("failed to complete pending task: %w", err)
	}
	return nil
}

// MarkTaskOpened stamps opened_at the first time only.
func (s *Store) MarkTaskOpened(id int64, profileID string) error {
	if _, err := s.exec(`UPDATE pending_tasks SET opened_at = ? WHERE id = ? AND profile_id = ? AND opened_at IS NULL`, time.Now(), id, normalizeProfileID(profileID)); err != nil {
		return fmt.Errorf("failed to mark task opened: %w", err)
	}
	return nil
}

// GetPendingTaskStats counts a profile's tasks by status.
func (s *Store) GetPendingTaskStats(profileID string) (pending, completed, skipped int, err error) {
	var p, c, k sql.NullInt64
	err = s.queryRow(`SELECT SUM(CASE WHEN status='pending' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END), SUM(CASE WHEN status='skipped' THEN 1 ELSE 0 END)
		FROM pending_tasks WHERE profile_id = ?`, normalizeProfileID(profileID)).Scan(&p, &c, &k)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get task stats: %w", err)
	}
	return int(p.Int64), int(c.Int64), int(k.Int64), nil
}

// GetFormsWithStatus lists the opt-out forms found in a profile's replies,
// one per broker (the latest), with how far each has got.
func (s *Store) GetFormsWithStatus(profileID string) ([]FormWithStatus, error) {
	latest, err := s.LatestRequests(profileID)
	if err != nil {
		return nil, err
	}
	stage := make(map[string]PipelineStatus, len(latest))
	for _, r := range latest {
		stage[r.BrokerID] = r.PipelineStatus
	}

	// the task join is pinned to the profile, so another profile's task for
	// the same broker never matches
	rows, err := s.query(`
		SELECT br.id, br.broker_id, br.broker_name, br.form_url, br.email_subject, br.created_at,
			COALESCE(pt.id, 0), COALESCE(pt.status, '')
		FROM broker_responses br
		LEFT JOIN pending_tasks pt ON br.broker_id = pt.broker_id AND pt.profile_id = br.profile_id AND pt.task_type IN ('captcha', 'manual_form')
		WHERE br.profile_id = ? AND br.form_url IS NOT NULL AND br.form_url != ''
		ORDER BY br.created_at DESC, br.id DESC, CASE WHEN pt.status = 'pending' THEN 0 ELSE 1 END, pt.id DESC`,
		normalizeProfileID(profileID))
	if err != nil {
		return nil, fmt.Errorf("failed to query forms: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var forms []FormWithStatus
	seen := map[string]bool{}
	for rows.Next() {
		var f FormWithStatus
		var taskStatus string
		if err := rows.Scan(&f.ResponseID, &f.BrokerID, &f.BrokerName, &f.FormURL, &f.EmailSubject, &f.DetectedAt, &f.TaskID, &taskStatus); err != nil {
			return nil, fmt.Errorf("failed to scan form: %w", err)
		}
		if seen[f.BrokerID] {
			continue
		}
		seen[f.BrokerID] = true
		f.PipelineStatus = stage[f.BrokerID]
		f.Status = formStatus(f.PipelineStatus, taskStatus, f.TaskID)
		forms = append(forms, f)
	}
	return forms, rows.Err()
}

// formStatus decides what a form's state is. The task's own status wins over
// the pipeline stage, except that a broker with nothing on file resolves it.
func formStatus(stage PipelineStatus, taskStatus string, taskID int64) string {
	switch {
	case stage == PipelineNoData:
		return "resolved"
	case taskStatus == "completed":
		return "filled"
	case taskStatus == "skipped":
		return "skipped"
	case taskStatus == "pending" && taskID > 0:
		return "captcha"
	case stage == PipelineFormFilled || stage == PipelineConfirmed:
		return "filled"
	case stage == PipelineFailed:
		return "failed"
	case stage == PipelineRejected:
		return "skipped"
	}
	return "pending"
}

// GetFormStats counts a profile's forms by status.
func (s *Store) GetFormStats(profileID string) (pending, filled, captcha, failed, skipped int, err error) {
	forms, err := s.GetFormsWithStatus(profileID)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	for _, f := range forms {
		switch f.Status {
		case "pending":
			pending++
		case "filled":
			filled++
		case "captcha":
			captcha++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}
	return
}
