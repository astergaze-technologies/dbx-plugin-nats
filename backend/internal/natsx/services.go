package natsx

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

type Service struct {
	Name        string            `json:"name"`
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Started     time.Time         `json:"started"`
	Endpoints   []ServiceEndpoint `json:"endpoints"`
}

type ServiceEndpoint struct {
	Name       string  `json:"name"`
	Subject    string  `json:"subject"`
	QueueGroup string  `json:"queueGroup,omitempty"`
	Requests   int     `json:"requests"`
	Errors     int     `json:"errors"`
	LastError  string  `json:"lastError,omitempty"`
	AvgMillis  float64 `json:"avgMs"`
}

// discoveryWait is how long to collect replies from running service instances.
const discoveryWait = time.Second

// Services discovers NATS micro services via the $SRV.INFO and $SRV.STATS verbs.
func (m *Manager) Services(connectionID string) ([]Service, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	infos, err := collect[micro.Info](s.nc, "$SRV.INFO")
	if err != nil {
		return nil, err
	}
	stats, err := collect[micro.Stats](s.nc, "$SRV.STATS")
	if err != nil {
		return nil, err
	}
	statsByID := map[string]micro.Stats{}
	for _, st := range stats {
		statsByID[st.ID] = st
	}
	out := make([]Service, 0, len(infos))
	for _, info := range infos {
		st := statsByID[info.ID]
		svc := Service{Name: info.Name, ID: info.ID, Version: info.Version, Description: info.Description,
			Metadata: info.Metadata, Started: st.Started, Endpoints: []ServiceEndpoint{}}
		for _, ep := range info.Endpoints {
			e := ServiceEndpoint{Name: ep.Name, Subject: ep.Subject, QueueGroup: ep.QueueGroup}
			for _, es := range st.Endpoints {
				if es.Name == ep.Name {
					e.Requests, e.Errors, e.LastError = es.NumRequests, es.NumErrors, es.LastError
					e.AvgMillis = float64(es.AverageProcessingTime.Microseconds()) / 1000
				}
			}
			svc.Endpoints = append(svc.Endpoints, e)
		}
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// collect publishes a discovery request and gathers every reply until discoveryWait passes.
func collect[T any](nc *nats.Conn, subject string) ([]T, error) {
	inbox := nc.NewRespInbox()
	sub, err := nc.SubscribeSync(inbox)
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()
	if err := nc.PublishRequest(subject, inbox, nil); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(discoveryWait)
	var out []T
	for {
		msg, err := sub.NextMsg(time.Until(deadline))
		if err != nil {
			return out, nil
		}
		var v T
		if json.Unmarshal(msg.Data, &v) == nil {
			out = append(out, v)
		}
	}
}
