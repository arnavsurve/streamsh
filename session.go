package streamsh

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session represents an active or recently disconnected shell session.
type Session struct {
	ID           uuid.UUID
	ShortID      string
	Title        string
	CreatedAt    time.Time
	LastActivity time.Time
	LastCommand  string
	Connected    bool
	Buffer       *RingBuffer
	Collab       bool
	clientConn   net.Conn
	connMu       sync.Mutex
}

// Store is a thread-safe collection of sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*Session
}

// NewStore creates an empty session store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[uuid.UUID]*Session),
	}
}

// Create adds a new session to the store and returns it.
func (s *Store) Create(title string, bufCap int, collab bool, conn net.Conn) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New()
	now := time.Now()
	sess := &Session{
		ID:           id,
		ShortID:      id.String()[:8],
		Title:        title,
		CreatedAt:    now,
		LastActivity: now,
		Connected:    true,
		Buffer:       NewRingBuffer(bufCap),
		Collab:       collab,
		clientConn:   conn,
	}
	s.sessions[id] = sess
	return sess
}

// SendInput sends text to the session's PTY via the client connection.
func (s *Session) SendInput(text string) error {
	if !s.Collab {
		return fmt.Errorf("session %s is not collaborative (start with --collab)", s.ShortID)
	}
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if !s.Connected || s.clientConn == nil {
		return fmt.Errorf("session %s is not connected", s.ShortID)
	}
	env := Envelope{
		Type:    MsgInput,
		Payload: mustMarshal(InputPayload{Text: text}),
	}
	return json.NewEncoder(s.clientConn).Encode(env)
}

// ClearConn removes the client connection reference.
func (s *Session) ClearConn() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.clientConn = nil
}

// Get returns a session by its full UUID.
func (s *Store) Get(id uuid.UUID) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// FindByPrefix finds a session whose ShortID or full UUID string starts with prefix.
// Returns an error if the prefix matches zero or multiple sessions.
func (s *Store) FindByPrefix(prefix string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix = strings.ToLower(prefix)
	var match *Session
	for _, sess := range s.sessions {
		if strings.HasPrefix(strings.ToLower(sess.ID.String()), prefix) ||
			strings.HasPrefix(strings.ToLower(sess.ShortID), prefix) {
			if match != nil {
				return nil, fmt.Errorf("ambiguous prefix %q: matches multiple sessions", prefix)
			}
			match = sess
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no session found with prefix %q", prefix)
	}
	return match, nil
}

// FindByTitle finds a session with an exact (case-insensitive) title match.
func (s *Store) FindByTitle(title string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(title)
	for _, sess := range s.sessions {
		if strings.ToLower(sess.Title) == lower {
			return sess, nil
		}
	}
	return nil, fmt.Errorf("no session found with title %q", title)
}

// Resolve finds a session by UUID, short ID prefix, or title.
func (s *Store) Resolve(identifier string) (*Session, error) {
	// Try UUID first
	if id, err := uuid.Parse(identifier); err == nil {
		if sess, ok := s.Get(id); ok {
			return sess, nil
		}
		return nil, fmt.Errorf("no session found with ID %s", id)
	}

	// Try prefix match
	if sess, err := s.FindByPrefix(identifier); err == nil {
		return sess, nil
	}

	// Try title match
	if sess, err := s.FindByTitle(identifier); err == nil {
		return sess, nil
	}

	return nil, fmt.Errorf("no session found matching %q", identifier)
}

// Remove deletes a session from the store.
func (s *Store) Remove(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// List returns all sessions.
func (s *Store) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, sess)
	}
	return result
}
