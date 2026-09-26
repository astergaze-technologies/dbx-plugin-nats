package natsx

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

// Auth modes offered by the connection form.
const (
	AuthNone     = "none"
	AuthPassword = "password"
	AuthToken    = "token"
	AuthCreds    = "creds"
)

// ErrNotConnected is returned when a workbench call references a connection
// that DBX has not opened (or has already closed).
var ErrNotConnected = errors.New("connection is not open; open the saved NATS connection first")

// Config is everything needed to dial one NATS server.
type Config struct {
	Name        string
	Host        string
	Port        int
	Auth        string
	Username    string
	Password    string
	Token       string
	Creds       string // decorated .creds file content
	CredsPath   string // path to a .creds file (desktop hosts)
	TLS         bool
	TLSInsecure bool
	Timeout     time.Duration
}

// URL returns the server URL without credentials.
func (c Config) URL() string {
	scheme := "nats"
	if c.TLS {
		scheme = "tls"
	}
	return scheme + "://" + net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c Config) options() ([]nats.Option, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	name := "DBX"
	if c.Name != "" {
		name = "DBX · " + c.Name
	}
	opts := []nats.Option{
		nats.Name(name),
		nats.Timeout(timeout),
		// DBX may reach the server through a single tunnelled endpoint, so never
		// dial cluster peers the server advertises.
		nats.IgnoreDiscoveredServers(),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2 * time.Second),
	}
	if c.TLS && c.TLSInsecure {
		opts = append(opts, nats.Secure(&tls.Config{InsecureSkipVerify: true})) //nolint:gosec // explicit user choice
	}
	switch c.Auth {
	case "", AuthNone:
	case AuthPassword:
		if c.Username == "" {
			return nil, errors.New("username is required for user/password authentication")
		}
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	case AuthToken:
		if c.Token == "" {
			return nil, errors.New("token is required for token authentication")
		}
		opts = append(opts, nats.Token(c.Token))
	case AuthCreds:
		creds := c.Creds
		if creds == "" && c.CredsPath != "" {
			data, err := os.ReadFile(c.CredsPath)
			if err != nil {
				return nil, fmt.Errorf("read credentials file: %w", err)
			}
			creds = string(data)
		}
		if creds == "" {
			return nil, errors.New("a .creds file is required for credentials authentication")
		}
		userJWT, err := jwt.ParseDecoratedJWT([]byte(creds))
		if err != nil {
			return nil, errors.New("credentials file does not contain a user JWT")
		}
		keyPair, err := nkeys.ParseDecoratedUserNKey([]byte(creds))
		if err != nil {
			return nil, errors.New("credentials file does not contain a user seed")
		}
		opts = append(opts, nats.UserJWT(
			func() (string, error) { return userJWT, nil },
			func(nonce []byte) ([]byte, error) { return keyPair.Sign(nonce) },
		))
	default:
		return nil, fmt.Errorf("unknown authentication mode %q", c.Auth)
	}
	return opts, nil
}

// Dial opens a connection. Callers own closing it.
func Dial(c Config) (*nats.Conn, error) {
	if c.Host == "" || c.Port <= 0 {
		return nil, errors.New("host and port are required")
	}
	opts, err := c.options()
	if err != nil {
		return nil, err
	}
	nc, err := nats.Connect(c.URL(), opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", c.URL(), err)
	}
	return nc, nil
}

type session struct {
	nc   *nats.Conn
	js   jetstream.JetStream
	subs map[string]*subscription
}

// Manager owns one NATS session per DBX connection ID.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*session{}}
}

// Test dials, measures a round trip and closes again.
func Test(c Config) (string, error) {
	nc, err := Dial(c)
	if err != nil {
		return "", err
	}
	defer nc.Close()
	rtt, err := nc.RTT()
	if err != nil {
		return "", fmt.Errorf("server did not answer a ping: %w", err)
	}
	return fmt.Sprintf("Connected to %s (NATS %s), round trip %s",
		nc.ConnectedServerName(), nc.ConnectedServerVersion(), rtt.Round(time.Microsecond)), nil
}

// Connect opens a session for connectionID. Reconnecting an open ID replaces it.
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
	m.sessions[connectionID] = &session{nc: nc, js: js, subs: map[string]*subscription{}}
	m.mu.Unlock()
	if old != nil {
		old.close()
	}
	return nil
}

// Disconnect closes the session and its subscriptions. Unknown IDs are a no-op.
func (m *Manager) Disconnect(connectionID string) {
	m.mu.Lock()
	s := m.sessions[connectionID]
	delete(m.sessions, connectionID)
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

// CloseAll disconnects every session (used on shutdown).
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
	s := m.sessions[connectionID]
	if s == nil {
		return nil, ErrNotConnected
	}
	return s, nil
}

func (s *session) close() {
	for _, sub := range s.subs {
		sub.stop("disconnected")
	}
	s.nc.Close()
}
