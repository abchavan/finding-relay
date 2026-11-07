// Package relay turns scanner findings into tickets and chat notifications.
package relay

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"northwick.example/finding-relay/internal/chat"
	"northwick.example/finding-relay/internal/finding"
	"northwick.example/finding-relay/internal/ticket"
)

const (
	retryDelay     = 10 * time.Second
	rateLimitDelay = 5 * time.Second
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
//
// The ticket platform is the source of truth for "already ticketed": before
// creating a ticket the handler looks one up by external_ref (the
// fingerprint). The in-memory maps only save API calls and remember a chat
// post that still has to go out.
type Handler struct {
	log        *slog.Logger
	tickets    *ticket.Client
	chat       *chat.Client
	suppressed map[string]bool

	mu      sync.Mutex
	done    map[string]string // fingerprint -> ticket id, ticket and chat both sent
	pending map[string]string // fingerprint -> ticket id ("" if the create outcome is unknown), chat post still owed
}

// NewHandler wires a Handler.
func NewHandler(log *slog.Logger, tickets *ticket.Client, chat *chat.Client, suppressed map[string]bool) *Handler {
	return &Handler{
		log: log, tickets: tickets, chat: chat, suppressed: suppressed,
		done: map[string]string{}, pending: map[string]string{},
	}
}

// Handle processes a single finding message.
func (h *Handler) Handle(ctx context.Context, m Msg) {
	f, err := finding.Parse(m.Data())
	if errors.Is(err, finding.ErrBadLine) {
		h.log.Warn("finding breaks the contract, line ignored", "subject", m.Subject(), "fingerprint", f.Fingerprint)
		err = nil
	}
	if err != nil {
		// Redelivering a payload that does not parse will never succeed.
		h.log.Error("bad finding, dropping", "subject", m.Subject(), "err", err)
		_ = m.Term()
		return
	}
	log := h.log.With("fingerprint", f.Fingerprint, "id", f.ID, "scanner", f.Scanner,
		"severity", f.Severity, "repo", f.Repo)
	log.Info("finding received")

	if h.suppressed[f.Fingerprint] {
		log.Info("finding suppressed")
		_ = m.Ack()
		return
	}
	if h.isDone(f.Fingerprint) {
		_ = m.Ack()
		return
	}

	id, owed := h.pendingChat(f.Fingerprint)
	if id == "" {
		id, err = h.tickets.FindByExternalRef(ctx, f.Fingerprint)
		if err != nil {
			h.retry(log, m, "ticket lookup failed", err)
			return
		}
		switch {
		case id != "" && !owed:
			log.Info("ticket already exists", "ticket", id)
			h.markDone(f.Fingerprint, id)
			_ = m.Ack()
			return
		case id == "":
			// Remember the attempt: a create that times out or errors may
			// still have opened the ticket, and then we owe the chat post.
			h.setPending(f.Fingerprint, "")
			id, err = h.tickets.Create(ctx, ticket.FromFinding(f))
			if err != nil {
				h.retry(log, m, "ticket create failed", err)
				return
			}
			log.Info("ticket created", "ticket", id)
		}
		owed = true
		h.setPending(f.Fingerprint, id)
	}

	if owed {
		if err := h.chat.Post(ctx, chat.FromFinding(f, id)); err != nil {
			log.Error("chat webhook failed, will retry", "ticket", id, "err", err)
			_ = m.NakWithDelay(retryDelay)
			return
		}
	}
	h.markDone(f.Fingerprint, id)
	_ = m.Ack()
}

func (h *Handler) retry(log *slog.Logger, m Msg, msg string, err error) {
	if errors.Is(err, ticket.ErrRateLimited) {
		log.Warn("ticket API rate limited, will retry")
		_ = m.NakWithDelay(rateLimitDelay)
		return
	}
	log.Error(msg+", will retry", "err", err)
	_ = m.NakWithDelay(retryDelay)
}

func (h *Handler) isDone(fp string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.done[fp]
	return ok
}

func (h *Handler) pendingChat(fp string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id, ok := h.pending[fp]
	return id, ok
}

func (h *Handler) setPending(fp, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pending[fp] = id
}

func (h *Handler) markDone(fp, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pending, fp)
	h.done[fp] = id
}
