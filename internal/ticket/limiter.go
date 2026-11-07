package ticket

import (
	"context"
	"sync"
	"time"
)

// limiter spaces requests evenly so the client stays under the platform's
// per-token rate limit.
type limiter struct {
	mu    sync.Mutex
	every time.Duration
	next  time.Time
}

func newLimiter(perSecond float64) *limiter {
	if perSecond <= 0 {
		return &limiter{}
	}
	return &limiter{every: time.Duration(float64(time.Second) / perSecond)}
}

func (l *limiter) wait(ctx context.Context) error {
	if l.every == 0 {
		return nil
	}
	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	at := l.next
	l.next = l.next.Add(l.every)
	l.mu.Unlock()

	d := time.Until(at)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
