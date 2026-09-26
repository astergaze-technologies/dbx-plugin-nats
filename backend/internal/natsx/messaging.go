package natsx

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

func (m *Manager) Publish(connectionID, subject, data string, headers map[string]string) error {
	s, err := m.session(connectionID)
	if err != nil {
		return err
	}
	if err := ValidateSubject(subject, false); err != nil {
		return err
	}
	if err := s.nc.PublishMsg(&nats.Msg{Subject: subject, Data: []byte(data), Header: toHeader(headers)}); err != nil {
		return err
	}
	return s.nc.FlushTimeout(5 * time.Second)
}

type Reply struct {
	Subject   string            `json:"subject"`
	Headers   map[string]string `json:"headers,omitempty"`
	ElapsedMs float64           `json:"elapsedMs"`
	Payload
}

const MaxRequestTimeout = 60 * time.Second

func (m *Manager) Request(connectionID, subject, data string, headers map[string]string, timeout time.Duration) (*Reply, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	if err := ValidateSubject(subject, false); err != nil {
		return nil, err
	}
	if timeout <= 0 || timeout > MaxRequestTimeout {
		timeout = 5 * time.Second
	}
	start := time.Now()
	msg, err := s.nc.RequestMsg(&nats.Msg{Subject: subject, Data: []byte(data), Header: toHeader(headers)}, timeout)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, fmt.Errorf("no service is listening on %q", subject)
		}
		return nil, err
	}
	return &Reply{Subject: msg.Subject, Headers: flattenHeaders(msg.Header),
		ElapsedMs: float64(time.Since(start).Microseconds()) / 1000, Payload: encodePayload(msg.Data)}, nil
}
