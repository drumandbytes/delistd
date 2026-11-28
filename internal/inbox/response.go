package inbox

// ResponseType is what a broker's reply amounts to.
type ResponseType string

const (
	ResponseFormRequired         ResponseType = "form_required"         // wants a web form filled in
	ResponseConfirmationRequired ResponseType = "confirmation_required" // wants a link clicked or a reply sent
	ResponseSuccess              ResponseType = "success"               // removal done
	ResponseNoData               ResponseType = "no_data_held"          // nothing on file for you
	ResponseRejected             ResponseType = "rejected"              // refused
	ResponsePending              ResponseType = "pending"               // acknowledged, still working
	ResponseBounced              ResponseType = "bounced"               // the request never arrived
	ResponseUnknown              ResponseType = "unknown"               // a person has to look
)

// ClassifiedResponse is an email plus the verdict on it.
type ClassifiedResponse struct {
	Email            *Email
	Type             ResponseType
	URLs             ExtractedURLs
	FormURL          string // set when the reply points at a form
	ConfirmURL       string // set when the reply carries a confirmation link
	BouncedRecipient string // the address that bounced, for ResponseBounced
	Confidence       float64
	Reason           string
	NeedsReview      bool
}

// Summary counts classified responses by type.
type Summary struct {
	Total           int
	Success         int
	FormRequired    int
	ConfirmRequired int
	Rejected        int
	NoData          int
	Pending         int
	Bounced         int
	Unknown         int
	NeedReview      int
}

// SummarizeResponses tallies responses by type and by how many need review.
func SummarizeResponses(responses []ClassifiedResponse) Summary {
	s := Summary{Total: len(responses)}
	counter := map[ResponseType]*int{
		ResponseSuccess:              &s.Success,
		ResponseFormRequired:         &s.FormRequired,
		ResponseConfirmationRequired: &s.ConfirmRequired,
		ResponseRejected:             &s.Rejected,
		ResponseNoData:               &s.NoData,
		ResponsePending:              &s.Pending,
		ResponseBounced:              &s.Bounced,
		ResponseUnknown:              &s.Unknown,
	}
	for _, r := range responses {
		if n := counter[r.Type]; n != nil {
			*n++
		}
		if r.NeedsReview {
			s.NeedReview++
		}
	}
	return s
}

var reasons = map[ResponseType]string{
	ResponseSuccess:              "Email indicates removal request was completed successfully",
	ResponseNoData:               "Broker reports holding no matching personal data",
	ResponseFormRequired:         "Email contains link to opt-out form that needs to be filled",
	ResponseConfirmationRequired: "Email contains confirmation link that needs to be clicked",
	ResponseRejected:             "Broker rejected or could not process the removal request",
	ResponsePending:              "Request is being processed, follow-up may be needed",
	ResponseUnknown:              "Could not automatically classify this response",
}

func reasonFor(t ResponseType) string {
	if r, ok := reasons[t]; ok {
		return r
	}
	return "Unknown classification"
}
