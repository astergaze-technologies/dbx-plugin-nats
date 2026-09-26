package natsx

import "context"

type Overview struct {
	Server    ServerInfo    `json:"server"`
	JetStream *JetStreamUse `json:"jetstream,omitempty"`
	ReadOnly  bool          `json:"readOnly"`
}

type ServerInfo struct {
	Name       string  `json:"name"`
	ID         string  `json:"id"`
	Version    string  `json:"version"`
	Cluster    string  `json:"cluster,omitempty"`
	URL        string  `json:"url"`
	RTTMillis  float64 `json:"rttMs"`
	MaxPayload int64   `json:"maxPayload"`
	Headers    bool    `json:"headers"`
	TLS        bool    `json:"tls"`
}

type JetStreamUse struct {
	Memory    uint64 `json:"memory"`
	Storage   uint64 `json:"storage"`
	Streams   int    `json:"streams"`
	Consumers int    `json:"consumers"`
	MaxMemory int64  `json:"maxMemory"`
	MaxStore  int64  `json:"maxStore"`
}

func (m *Manager) Overview(ctx context.Context, connectionID string) (*Overview, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	rtt, _ := s.nc.RTT()
	_, tlsErr := s.nc.TLSConnectionState()
	out := &Overview{ReadOnly: s.readOnly, Server: ServerInfo{
		Name:       s.nc.ConnectedServerName(),
		ID:         s.nc.ConnectedServerId(),
		Version:    s.nc.ConnectedServerVersion(),
		Cluster:    s.nc.ConnectedClusterName(),
		URL:        s.nc.ConnectedUrlRedacted(),
		RTTMillis:  float64(rtt.Microseconds()) / 1000,
		MaxPayload: s.nc.MaxPayload(),
		Headers:    s.nc.HeadersSupported(),
		TLS:        tlsErr == nil,
	}}
	if info, err := s.js.AccountInfo(ctx); err == nil {
		out.JetStream = &JetStreamUse{
			Memory: info.Memory, Storage: info.Store,
			Streams: info.Streams, Consumers: info.Consumers,
			MaxMemory: info.Limits.MaxMemory, MaxStore: info.Limits.MaxStore,
		}
	}
	return out, nil
}
