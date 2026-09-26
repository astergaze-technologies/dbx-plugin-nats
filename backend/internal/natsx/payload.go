package natsx

import (
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// MaxPayloadBytes caps message bodies sent to the UI.
const MaxPayloadBytes = 64 * 1024

type Payload struct {
	Data      string `json:"data"`
	Encoding  string `json:"encoding"` // utf8 | base64
	Size      int    `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

func encodePayload(data []byte) Payload {
	p := Payload{Size: len(data), Encoding: "utf8"}
	if len(data) > MaxPayloadBytes {
		data, p.Truncated = data[:MaxPayloadBytes], true
	}
	if utf8.Valid(data) {
		p.Data = string(data)
	} else {
		p.Encoding, p.Data = "base64", base64.StdEncoding.EncodeToString(data)
	}
	return p
}

func flattenHeaders(h nats.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = strings.Join(v, ", ")
	}
	return out
}

func toHeader(h map[string]string) nats.Header {
	if len(h) == 0 {
		return nil
	}
	out := nats.Header{}
	for k, v := range h {
		out.Set(k, v)
	}
	return out
}

// ValidateSubject rejects malformed subjects; wildcards only when allowed.
func ValidateSubject(subject string, allowWildcards bool) error {
	if subject == "" {
		return errors.New("subject is required")
	}
	if strings.ContainsAny(subject, " \t\r\n") {
		return errors.New("subject must not contain whitespace")
	}
	tokens := strings.Split(subject, ".")
	for i, t := range tokens {
		switch {
		case t == "":
			return errors.New("subject has an empty token")
		case t == ">" && i != len(tokens)-1:
			return errors.New("'>' is only allowed as the last token")
		case (t == "*" || t == ">") && !allowWildcards:
			return errors.New("wildcards are not allowed here")
		}
	}
	return nil
}

// jetStreamUnavailable: servers without JetStream answer API calls with no responders.
func jetStreamUnavailable(err error) bool {
	return errors.Is(err, jetstream.ErrJetStreamNotEnabled) ||
		errors.Is(err, jetstream.ErrJetStreamNotEnabledForAccount) ||
		errors.Is(err, nats.ErrNoResponders)
}
