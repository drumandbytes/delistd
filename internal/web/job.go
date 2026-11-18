package web

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"maps"
	"sync"
	"time"
)

// JobStatus is where a send job stands.
type JobStatus string

const (
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusPaused    JobStatus = "paused" // the daily limit was reached
	JobStatusError     JobStatus = "error"  // stopped on an auth or config error
)

// jobRetention is how long a finished job stays in memory. Jobs are kept
// nowhere else; a week is plenty to look back at a send.
const jobRetention = 7 * 24 * time.Hour

// Job is one background send over a list of brokers. Callers go through its
// methods, which hold mu; JSON encoding takes a consistent snapshot. The JSON
// names are what the page's status poll reads.
type Job struct {
	ID        string    `json:"id"`
	ProfileID string    `json:"profile_id"`
	Status    JobStatus `json:"status"`

	Total    int `json:"total"`
	Sent     int `json:"sent"`
	Failed   int `json:"failed"`
	Progress int `json:"progress"` // percent of Total handled

	CurrentBroker   string `json:"current_broker"`
	CurrentBrokerID string `json:"current_broker_id"`

	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`

	Error      string `json:"error,omitempty"`
	ErrorType  string `json:"error_type,omitempty"`  // "auth", "rate_limit", ...
	DailyLimit int    `json:"daily_limit,omitempty"` // emails allowed per day
	DaySent    int    `json:"day_sent,omitempty"`    // emails sent today

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// locked runs fn with the job locked.
func (j *Job) locked(fn func()) {
	j.mu.Lock()
	defer j.mu.Unlock()
	fn()
}

// Update records progress. The broker ID lets the brokers page refresh only the
// row that changed rather than the whole list on every poll.
func (j *Job) Update(sent, failed int, broker, brokerID string) {
	j.locked(func() {
		j.Sent, j.Failed = sent, failed
		j.CurrentBroker, j.CurrentBrokerID = broker, brokerID
		if j.Total > 0 {
			j.Progress = (sent + failed) * 100 / j.Total
		}
	})
}

// finish stamps the end time and clears the broker being worked on.
func (j *Job) finish(status JobStatus) {
	j.Status, j.CompletedAt = status, time.Now()
	j.CurrentBroker, j.CurrentBrokerID = "", ""
}

// Complete marks the job done.
func (j *Job) Complete() {
	j.locked(func() {
		j.finish(JobStatusCompleted)
		j.Progress = 100
	})
}

// StopWithError ends the job early and keeps the reason. The status stays
// "completed"; the page tells the cases apart by ErrorType.
func (j *Job) StopWithError(errorType, message string) {
	j.locked(func() {
		j.finish(JobStatusCompleted)
		j.Error, j.ErrorType = message, errorType
	})
}

// Pause marks the job as stopped by the daily limit.
func (j *Job) Pause(daySent int, message string) {
	j.locked(func() { j.DaySent, j.Status, j.Error = daySent, JobStatusPaused, message })
}

// SetDailyLimit records the limit the job runs under.
func (j *Job) SetDailyLimit(limit int) { j.locked(func() { j.DailyLimit = limit }) }

// Cancel stops a running job and cancels its context. A job in any other state
// is left alone.
func (j *Job) Cancel() {
	j.locked(func() {
		if j.Status != JobStatusRunning {
			return
		}
		j.finish(JobStatusCancelled)
		if j.cancel != nil {
			j.cancel()
		}
	})
}

// GetStatus is the job's current status.
func (j *Job) GetStatus() JobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Status
}

// Context ends when the job is cancelled.
func (j *Job) Context() context.Context { return j.ctx }

// endedBefore reports a job that is no longer running and finished before cutoff.
func (j *Job) endedBefore(cutoff time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Status != JobStatusRunning && j.CompletedAt.Before(cutoff)
}

// MarshalJSON encodes a snapshot, so a poll never sees a job halfway through Update.
func (j *Job) MarshalJSON() ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	type snapshot Job // no methods, so no recursion
	return json.Marshal((*snapshot)(j))
}

// JobManager holds the jobs of one server process. It drops finished jobs older
// than jobRetention whenever it starts a new one, so there is no background
// goroutine to leak.
type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewJobManager() *JobManager { return &JobManager{jobs: make(map[string]*Job)} }

// add registers a new running job. The caller holds jm.mu.
func (jm *JobManager) add(total int, profileID string) *Job {
	jm.evict(jobRetention)
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{
		ID: rand.Text(), ProfileID: profileID, Status: JobStatusRunning,
		Total: total, StartedAt: time.Now(), ctx: ctx, cancel: cancel,
	}
	jm.jobs[j.ID] = j
	return j
}

// CreateIfNoActive starts a job unless the profile already has one running,
// deciding under one lock so two requests cannot both send. It returns the
// running job and false, or the new job and true.
func (jm *JobManager) CreateIfNoActive(total int, profileID string) (*Job, bool) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if j := jm.running(profileID); j != nil {
		return j, false
	}
	return jm.add(total, profileID), true
}

// running finds a running job of the profile (of any profile when profileID is
// empty). The caller holds jm.mu.
func (jm *JobManager) running(profileID string) *Job {
	for j := range maps.Values(jm.jobs) {
		if j.GetStatus() == JobStatusRunning && (profileID == "" || j.ProfileID == profileID) {
			return j
		}
	}
	return nil
}

// Get returns the job with that ID, or nil.
func (jm *JobManager) Get(id string) *Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[id]
}

// GetActive is the running job of one profile, or nil. It is per profile so two
// profiles can send at once, each within its own daily limit and history.
func (jm *JobManager) GetActive(profileID string) *Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.running(profileID)
}

// AnyActive reports whether any profile has a running job.
func (jm *JobManager) AnyActive() bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.running("") != nil
}

// evict drops finished jobs older than maxAge. The caller holds jm.mu.
func (jm *JobManager) evict(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	maps.DeleteFunc(jm.jobs, func(_ string, j *Job) bool { return j.endedBefore(cutoff) })
}
