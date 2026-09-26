// Package natsx implements the NATS operations behind the DBX plugin.
package natsx

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

const (
	AuthNone     = "none"
	AuthPassword = "password"
	AuthToken    = "token"
	AuthCreds    = "creds"
)

type Config struct {
	Name        string
	Host        string
	Port        int
	Auth        string
	Username    string
	Password    string
	Token       string
	Creds       string
	CredsPath   string
	TLS         bool
	TLSInsecure bool
	Timeout     time.Duration
	ReadOnly    bool
}

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
		// DBX may tunnel a single endpoint; cluster peers would be unreachable.
		nats.IgnoreDiscoveredServers(),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2 * time.Second),
	}
	if c.TLS && c.TLSInsecure {
		opts = append(opts, nats.Secure(&tls.Config{InsecureSkipVerify: true})) //nolint:gosec // user opt-in
	}
	auth, err := c.authOption()
	if err != nil {
		return nil, err
	}
	if auth != nil {
		opts = append(opts, auth)
	}
	return opts, nil
}

func (c Config) authOption() (nats.Option, error) {
	switch c.Auth {
	case "", AuthNone:
		return nil, nil
	case AuthPassword:
		if c.Username == "" {
			return nil, errors.New("username is required for user/password authentication")
		}
		return nats.UserInfo(c.Username, c.Password), nil
	case AuthToken:
		if c.Token == "" {
			return nil, errors.New("token is required for token authentication")
		}
		return nats.Token(c.Token), nil
	case AuthCreds:
		return c.credsOption()
	}
	return nil, fmt.Errorf("unknown authentication mode %q", c.Auth)
}

func (c Config) credsOption() (nats.Option, error) {
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
	return nats.UserJWT(
		func() (string, error) { return userJWT, nil },
		func(nonce []byte) ([]byte, error) { return keyPair.Sign(nonce) },
	), nil
}

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

// Test dials, measures one round trip and closes.
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
