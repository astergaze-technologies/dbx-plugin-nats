package natsx

import (
	"errors"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var (
	ErrNotConnected = errors.New("connection is not open; open the saved NATS connection first")
	ErrReadOnly     = errors.New("this connection is read-only")
)

type session struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	readOnly bool
	live     map[string]*liveFeed
}

func (s *session) close() {
	for _, feed := range s.live {
		feed.stop("disconnected")
	}
	s.nc.Close()
}

// Manager owns one NATS session per DBX connection ID.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*session{}}
}

// Connect opens a session; connecting an open ID replaces it.
func (m *Manager) Connect(connectionID string, c Config) error {
	nc, err := Dial(c)
	if err != nil {
		return err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return err
	}
	m.mu.Lock()
	old := m.sessions[connectionID]
	m.sessions[connectionID] = &session{nc: nc, js: js, readOnly: c.ReadOnly, live: map[string]*liveFeed{}}
	m.mu.Unlock()
	if old != nil {
		old.close()
	}
	return nil
}

func (m *Manager) Disconnect(connectionID string) {
	m.mu.Lock()
	s := m.sessions[connectionID]
	delete(m.sessions, connectionID)
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Disconnect(id)
	}
}

func (m *Manager) session(connectionID string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[connectionID]; s != nil {
		return s, nil
	}
	return nil, ErrNotConnected
}

func (m *Manager) writable(connectionID string) (*session, error) {
	s, err := m.session(connectionID)
	if err == nil && s.readOnly {
		return nil, ErrReadOnly
	}
	return s, err
}
