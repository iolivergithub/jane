package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// SessionStore holds Web UI login sessions, keyed by an opaque random token held in a cookie.
//
// The default implementation is in memory, so a restart of Jane logs everybody out.
// A persistent store (e.g. a Mongo collection) can be dropped in behind this interface.
type SessionStore interface {
	Create(username string) (token string, err error)
	// Lookup returns the user for a token and refreshes its expiry (sliding timeout).
	Lookup(token string) (username string, ok bool)
	Delete(token string)
}

type session struct {
	username string
	expires  time.Time
}

type memorySessionStore struct {
	mu       sync.Mutex
	timeout  time.Duration
	sessions map[string]session
	now      func() time.Time
}

func newMemorySessionStore(timeout time.Duration) *memorySessionStore {
	s := &memorySessionStore{
		timeout:  timeout,
		sessions: make(map[string]session),
		now:      time.Now,
	}
	go s.janitor()
	return s
}

func (s *memorySessionStore) Create(username string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = session{username: username, expires: s.now().Add(s.timeout)}
	return token, nil
}

func (s *memorySessionStore) Lookup(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[token]
	if !ok {
		return "", false
	}
	now := s.now()
	if now.After(sess.expires) {
		delete(s.sessions, token)
		return "", false
	}
	sess.expires = now.Add(s.timeout)
	s.sessions[token] = sess
	return sess.username, true
}

func (s *memorySessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// janitor periodically removes expired sessions so the map does not grow without bound.
func (s *memorySessionStore) janitor() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := s.now()
		for t, sess := range s.sessions {
			if now.After(sess.expires) {
				delete(s.sessions, t)
			}
		}
		s.mu.Unlock()
	}
}
