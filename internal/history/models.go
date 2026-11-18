package history

import (
	"database/sql"
	"time"
)

// Status is whether a removal email went out.
type Status string

const (
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
	StatusPending Status = "pending"
)

// DefaultProfileID mirrors config.DefaultProfileID (importing config would
// make a cycle). Rows from before profiles existed belong to it.
const DefaultProfileID = "default"

// PipelineStatus is a request's stage between "email sent" and an outcome.
type PipelineStatus string

const (
	PipelineEmailSent            PipelineStatus = "email_sent"
	PipelineAwaitingResponse     PipelineStatus = "awaiting_response"
	PipelineFormRequired         PipelineStatus = "form_required"
	PipelineFormFilled           PipelineStatus = "form_filled"
	PipelineAwaitingCaptcha      PipelineStatus = "awaiting_captcha"
	PipelineCaptchaSolved        PipelineStatus = "captcha_solved"
	PipelineAwaitingConfirmation PipelineStatus = "awaiting_confirmation"
	PipelineConfirmed            PipelineStatus = "confirmed"
	PipelineNoData               PipelineStatus = "no_data_held"
	PipelineFailed               PipelineStatus = "failed"
	PipelineRejected             PipelineStatus = "rejected"
)

// TaskType is the kind of work a pending task asks of a person.
type TaskType string

const (
	TaskCaptcha    TaskType = "captcha"
	TaskManualForm TaskType = "manual_form"
	TaskReview     TaskType = "review"
	TaskConfirm    TaskType = "confirm"
)

// Record is one removal request sent (or tried) to a broker.
type Record struct {
	ID             int64
	ProfileID      string
	BrokerID       string
	BrokerName     string
	Email          string
	Template       string
	Status         Status
	MessageID      string
	Error          string
	SentAt         time.Time
	CreatedAt      time.Time
	PipelineStatus PipelineStatus // where the request stands now
	PipelineActor  string         // "human", "mcp:<client>", or empty when automatic
	SentMethod     string         // "smtp" when Delistd sent it, "manual" when the user did
}

type BrokerResponse struct {
	ID           int64
	ProfileID    string
	BrokerID     string
	BrokerName   string
	ResponseType string // form_required, confirmation_required, success, no_data_held, rejected, pending, bounced or unknown
	EmailFrom    string
	EmailSubject string
	EmailBody    string // kept so a reply can be classified again
	FormURL      string // the opt-out form the reply points at, if any
	ConfirmURL   string // the confirmation link in the reply, if any
	Confidence   float64
	NeedsReview  bool
	ReceivedAt   time.Time
	ProcessedAt  time.Time
	CreatedAt    time.Time
}

// PendingTask is something a person has to do: solve a CAPTCHA, fill a form,
// look at a reply.
type PendingTask struct {
	ID             int64
	ProfileID      string
	BrokerID       string
	BrokerName     string
	TaskType       TaskType
	FormURL        string
	ScreenshotPath string
	BrowserState   string // JSON profile data for the helper page
	Notes          string
	Status         string // pending, completed or skipped
	CreatedAt      time.Time
	OpenedAt       sql.NullTime // when the helper page was first opened
	CompletedAt    sql.NullTime
	// Actor is "human", or "mcp:<client>" for work an agent recorded.
	Actor string
	// ReviewReason is set when an automated step needs a person to decide.
	ReviewReason string
}

// BrokerStatus summarises one broker's requests for a profile.
type BrokerStatus struct {
	BrokerID  string
	LastSent  time.Time
	Status    Status // of the latest request
	TotalSent int    // number of requests, not only successful ones
}

// FormWithStatus is an opt-out form found in a reply and how far it has got.
type FormWithStatus struct {
	ResponseID     int64
	BrokerID       string
	BrokerName     string
	FormURL        string
	EmailSubject   string
	DetectedAt     time.Time
	Status         string // pending, filled, captcha, failed, skipped or resolved
	TaskID         int64  // the task working on it, if any
	PipelineStatus PipelineStatus
}
