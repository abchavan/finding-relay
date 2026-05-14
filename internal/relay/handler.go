// Package relay turns scanner findings into tickets and chat notifications.
package relay

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"northwick.example/finding-relay/internal/chat"
	"northwick.example/finding-relay/internal/finding"
	"northwick.example/finding-relay/internal/ticket"
)

// Msg is the subset of a JetStream message the handler needs.
type Msg interface {
	Data() []byte
	Subject() string
	Ack() error
	Nak() error
	NakWithDelay(delay time.Duration) error
	Term() error
}

// Handler processes one message at a time.
type Handler struct {
	log     *slog.Logger
	tickets *ticket.Client
	chat    *chat.Client
}

// NewHandler wires a Handler.
func NewHandler(log *slog.Logger, tickets *ticket.Client, chat *chat.Client) *Handler {
	return &Handler{log: log, tickets: tickets, chat: chat}
}

// Handle processes a single finding message.
func (h *Handler) Handle(ctx context.Context, m Msg) {
	f, err := finding.Parse(m.Data())
	if err != nil {
		h.log.Error("bad finding, will retry", "err", err)
		_ = m.NakWithDelay(5 * time.Second)
		return
	}
	h.log.Info("finding received", "finding", f)
	id, err := h.tickets.Create(ctx, ticket.FromFinding(f))
	if err != nil {
		if errors.Is(err, ticket.ErrRateLimited) {
			h.log.Warn("ticket API rate limited, dropping", "fingerprint", f.Fingerprint)
			_ = m.Term()
			return
		}
		_ = m.Nak()
		return
	}
	if err := h.chat.Post(ctx, chat.FromFinding(f, id)); err != nil {
		h.log.Error("chat webhook failed, will retry", "err", err)
		_ = m.Nak()
		return
	}
	_ = m.Ack()
}
