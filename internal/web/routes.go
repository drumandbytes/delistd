package web

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type route struct {
	method, pattern string
	handler         http.HandlerFunc
}

func mount(r chi.Router, routes []route) {
	for _, rt := range routes {
		r.Method(rt.method, rt.pattern, rt.handler)
	}
}

const (
	mGet    = http.MethodGet
	mPost   = http.MethodPost
	mDelete = http.MethodDelete
)

func (s *Server) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.reloadConfigOnChange)
	r.Use(s.requireUnlock)
	r.Use(middleware.Compress(5))
	r.Use(s.requireAllowedHost)
	r.Use(securityHeaders)
	// CrossOriginProtection checks Sec-Fetch-Site (Origin as the fallback)
	// rather than tokens. A DNS-rebound name still looks same-origin to it,
	// which is why requireAllowedHost checks Host too.
	r.Use(http.NewCrossOriginProtection().Handler)

	assets, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))

	mount(r, s.pageRoutes())
	r.Route("/setup", func(r chi.Router) { mount(r, s.setupRoutes()) })
	r.Route("/api", func(r chi.Router) { mount(r, s.apiRoutes()) }) // fragments and actions for HTMX
	return r
}

func (s *Server) pageRoutes() []route {
	return []route{
		{mGet, "/", s.handleDashboard},
		{mGet, "/healthz", s.handleHealthz},

		{mGet, "/brokers", s.handleBrokers},
		{mGet, "/brokers/new", s.handleBrokerNew},
		{mPost, "/brokers/new", s.handleBrokerNew},
		{mGet, "/brokers/{brokerID}/email", s.handleBrokerEmail},

		{mGet, "/history", s.handleHistory},
		{mGet, "/export", s.handleExport},
		{mGet, "/export/drafts.zip", s.handleDraftsZip},

		{mGet, "/unlock", s.handleUnlock},
		{mPost, "/unlock", s.handleUnlock},
		{mPost, "/upgrade-notice/dismiss", s.handleDismissUpgradeNotice},

		{mGet, "/settings", s.handleSettings},
		{mPost, "/settings/passphrase", s.handleSettingsPassphrase},
		{mPost, "/settings/inbox", s.handleSettingsInbox},
		{mPost, "/settings/options", s.handleSettingsOptions},
		{mPost, "/settings/automation", s.handleSettingsAutomation},
		{mPost, "/settings/brokers/update", s.handleSettingsBrokersUpdate},
		{mGet, "/settings/profiles/new", s.handleSettingsProfileNew},
		{mPost, "/settings/profiles/new", s.handleSettingsProfileNew},
		{mGet, "/settings/profiles/{profileID}/edit", s.handleSettingsProfileEdit},
		{mPost, "/settings/profiles/{profileID}/edit", s.handleSettingsProfileEdit},
		{mPost, "/settings/profiles/{profileID}/delete", s.handleSettingsProfileDelete},

		{mGet, "/pipeline", s.handlePipeline},
		{mGet, "/pipeline/responses/{responseID}", s.handleResponseReview},
		{mGet, "/automation", s.handleAutomation},
		{mPost, "/automation/approve", s.handleAutomationApprove},

		{mGet, "/tasks", s.handleTasks},
		{mGet, "/tasks/{taskID}", s.handleTaskDetail},
		{mGet, "/tasks/{taskID}/screenshot", s.handleTaskScreenshot},
		{mGet, "/tasks/{taskID}/helper", s.handleTaskHelper},
		{mPost, "/tasks/{taskID}/complete", s.handleTaskComplete},
		{mPost, "/tasks/{taskID}/skip", s.handleTaskSkip},
		{mGet, "/forms", s.handleForms},
		{mPost, "/forms/{brokerID}/complete", s.handleFormComplete},
		{mPost, "/forms/{brokerID}/skip", s.handleFormSkip},

		{mPost, "/oauth/start", s.handleOAuthStart},
		{mGet, "/oauth/status", s.handleOAuthStatus},
	}
}

func (s *Server) setupRoutes() []route {
	return []route{
		{mGet, "/", s.handleSetupWelcome},
		{mGet, "/profile", s.handleSetupProfile},
		{mPost, "/profile", s.handleSetupProfile},
		{mGet, "/email", s.handleSetupEmail},
		{mPost, "/email", s.handleSetupEmail},
		{mGet, "/test", s.handleSetupTest},
		{mPost, "/test/send", s.handleSetupTestSend},
		{mGet, "/complete", s.handleSetupComplete},
	}
}

func (s *Server) apiRoutes() []route {
	return []route{
		{mGet, "/brokers", s.handleAPIBrokers},
		{mGet, "/brokers/{brokerID}/status", s.handleAPIBrokerStatus},
		{mPost, "/brokers/{brokerID}/exclude", s.handleAPIExcludeBroker},
		{mPost, "/brokers/{brokerID}/include", s.handleAPIIncludeBroker},
		{mPost, "/brokers/{brokerID}/mark-sent", s.handleAPIMarkSent},
		{mPost, "/brokers/{brokerID}/mark-bounced", s.handleAPIMarkBounced},

		{mDelete, "/history/failed", s.handleAPIDeleteFailed},
		{mDelete, "/history", s.handleAPIDeleteAllHistory},

		{mPost, "/send/{brokerID}", s.handleAPISendOne},
		{mPost, "/send-all", s.handleAPISendAll},
		{mGet, "/job/active", s.handleAPIJobActive},
		{mGet, "/job/{jobID}/status", s.handleAPIJobStatus},
		{mPost, "/job/{jobID}/cancel", s.handleAPIJobCancel},

		{mGet, "/pipeline/responses", s.handleAPIResponses},
		{mPost, "/pipeline/responses/{responseID}/reviewed", s.handleAPIResponseReviewed},
		{mPost, "/inbox/scan", s.handleAPIInboxScan},
		{mPost, "/inbox/rescan", s.handleAPIInboxRescan},
		{mPost, "/inbox/reclassify", s.handleAPIReclassify},
		{mPost, "/inbox/bounces", s.handleAPIBounces},
		{mPost, "/inbox/bounces/clear", s.handleAPIBouncesClear},
		{mPost, "/profile", s.handleAPISwitchProfile},
	}
}
