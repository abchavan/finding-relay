// Command relay consumes findings from JetStream and files tickets for them.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"northwick.example/finding-relay/internal/chat"
	"northwick.example/finding-relay/internal/config"
	"northwick.example/finding-relay/internal/relay"
	"northwick.example/finding-relay/internal/ticket"
)

type natsMsg struct{ m *nats.Msg }

func (n natsMsg) Data() []byte    { return n.m.Data }
func (n natsMsg) Subject() string { return n.m.Subject }
func (n natsMsg) Ack() error      { return n.m.Ack() }
func (n natsMsg) Nak() error      { return n.m.Nak() }
func (n natsMsg) NakWithDelay(d time.Duration) error {
	return n.m.NakWithDelay(d)
}
func (n natsMsg) Term() error { return n.m.Term() }

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("relay stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nc, err := nats.Connect(cfg.NATSURL, nats.UserInfo(cfg.NATSUser, cfg.NATSPass), nats.MaxReconnects(-1))
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}

	if _, err := js.StreamInfo(cfg.Stream); errors.Is(err, nats.ErrStreamNotFound) {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:       cfg.Stream,
			Subjects:   []string{cfg.Subjects},
			MaxAge:     168 * time.Hour,
			Duplicates: 2 * time.Minute,
			Storage:    nats.FileStorage,
		})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	ensureConsumer := func(policy nats.DeliverPolicy) error {
		if _, err := js.ConsumerInfo(cfg.Stream, cfg.Consumer); err == nil {
			return nil
		} else if !errors.Is(err, nats.ErrConsumerNotFound) {
			return err
		}
		_, err := js.AddConsumer(cfg.Stream, &nats.ConsumerConfig{
			Durable:       cfg.Consumer,
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       30 * time.Second,
			FilterSubject: cfg.Subjects,
			DeliverPolicy: policy,
		})
		return err
	}
	if err := ensureConsumer(nats.DeliverAllPolicy); err != nil {
		return err
	}

	h := relay.NewHandler(log, ticket.New(cfg.TicketsURL, cfg.TicketsTok), chat.New(cfg.ChatWebhook))
	subscribe := func() (*nats.Subscription, error) {
		return js.PullSubscribe(cfg.Subjects, cfg.Consumer, nats.Bind(cfg.Stream, cfg.Consumer))
	}
	sub, err := subscribe()
	if err != nil {
		return err
	}

	for ctx.Err() == nil {
		msgs, err := sub.Fetch(cfg.Batch, nats.MaxWait(5*time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			log.Warn("fetch failed, resubscribing", "err", err)
			time.Sleep(time.Second)
			if err := ensureConsumer(nats.DeliverAllPolicy); err != nil {
				log.Error("ensure consumer", "err", err)
				continue
			}
			if s, err := subscribe(); err == nil {
				sub = s
			}
			continue
		}
		for _, m := range msgs {
			h.Handle(ctx, natsMsg{m})
		}
	}
	return nil
}
