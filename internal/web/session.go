package web

import (
	"crypto/rand"
	"encoding/hex"
	"maps"
	"sync"
	"time"

	"github.com/drumandbytes/delistd/internal/config"
)

// Session is the setup wizard's state. It lives on the server so the mail
// password never reaches the browser, which holds only an opaque id.
type Session struct {
	ID         string
	Step       string
	Profile    config.Profile
	Email      config.EmailConfig
	ManualSend bool // sending by hand, so no SMTP account
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

func (s *Session) expired(now time.Time) bool { return now.After(s.ExpiresAt) }

// SessionStore keeps sessions in memory until they have been idle for ttl.
// Expired ones are dropped when looked up and whenever a new one is created, so
// no background goroutine is needed.
type SessionStore struct {
	mu       sync.Mutex
	byID     map[string]*Session
	idleTime time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{byID: make(map[string]*Session), idleTime: ttl}
}

// Create starts an empty session and returns its id (256 random bits).
func (s *SessionStore) Create() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	maps.DeleteFunc(s.byID, func(_ string, sess *Session) bool { return sess.expired(now) })
	s.byID[id] = &Session{ID: id, CreatedAt: now, ExpiresAt: now.Add(s.idleTime)}
	return id, nil
}

// live returns the unexpired session for id, dropping it if it has expired.
// The caller holds s.mu.
func (s *SessionStore) live(id string) *Session {
	sess, ok := s.byID[id]
	if !ok {
		return nil
	}
	if sess.expired(time.Now()) {
		delete(s.byID, id)
		return nil
	}
	return sess
}

// Get returns a copy of the session, or nil when there is none or it expired.
// It is a copy: changes go through Update, so a reader never sees a session
// another request is halfway through changing.
func (s *SessionStore) Get(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.live(id)
	if sess == nil {
		return nil
	}
	cp := *sess
	return &cp
}

// Update applies change to the session and restarts its idle timer. It reports
// false when there is no live session.
func (s *SessionStore) Update(id string, change func(*Session)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.live(id)
	if sess == nil {
		return false
	}
	change(sess)
	sess.ExpiresAt = time.Now().Add(s.idleTime)
	return true
}

// Delete forgets the session.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, id)
}
