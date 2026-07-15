// Package events publishes catalog change notifications on NATS.
package events

import (
	"encoding/json"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type natsPub struct{ nc *nats.Conn }

func NewNATS(nc *nats.Conn) *natsPub { return &natsPub{nc: nc} }

// Publish is fire-and-forget: catalog mutations must not fail on event errors.
func (p *natsPub) Publish(subject string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("event marshal", "subject", subject, "err", err)
		return
	}
	if err := p.nc.Publish(subject, data); err != nil {
		slog.Error("event publish", "subject", subject, "err", err)
	}
}
