package natsx

import (
	"context"
	"fmt"
	"time"
)

type ConsumerSummary struct {
	Name           string    `json:"name"`
	Durable        bool      `json:"durable"`
	Pull           bool      `json:"pull"`
	Description    string    `json:"description,omitempty"`
	FilterSubjects []string  `json:"filterSubjects"`
	DeliverPolicy  string    `json:"deliverPolicy"`
	AckPolicy      string    `json:"ackPolicy"`
	NumPending     uint64    `json:"numPending"`
	NumAckPending  int       `json:"numAckPending"`
	NumRedelivered int       `json:"numRedelivered"`
	NumWaiting     int       `json:"numWaiting"`
	DeliveredSeq   uint64    `json:"deliveredSeq"`
	AckFloorSeq    uint64    `json:"ackFloorSeq"`
	Created        time.Time `json:"created"`
}

func (m *Manager) Consumers(ctx context.Context, connectionID, stream string) ([]ConsumerSummary, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("stream %q: %w", stream, err)
	}
	out := []ConsumerSummary{}
	lister := st.ListConsumers(ctx)
	for info := range lister.Info() {
		c := info.Config
		filters := c.FilterSubjects
		if c.FilterSubject != "" {
			filters = append([]string{c.FilterSubject}, filters...)
		}
		out = append(out, ConsumerSummary{
			Name: info.Name, Durable: c.Durable != "", Pull: c.DeliverSubject == "",
			Description: c.Description, FilterSubjects: filters,
			DeliverPolicy: c.DeliverPolicy.String(), AckPolicy: c.AckPolicy.String(),
			NumPending: info.NumPending, NumAckPending: info.NumAckPending,
			NumRedelivered: info.NumRedelivered, NumWaiting: info.NumWaiting,
			DeliveredSeq: info.Delivered.Stream, AckFloorSeq: info.AckFloor.Stream, Created: info.Created,
		})
	}
	if err := lister.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *Manager) DeleteConsumer(ctx context.Context, connectionID, stream, name string) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	return s.js.DeleteConsumer(ctx, stream, name)
}
