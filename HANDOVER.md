# Handover: finding-relay

Dana Okafor, 2026-10-06

I'm moving to the ledger team next week, so the relay is yours. This is
everything I'd want to know if I were picking it up. It's a bit rushed, sorry;
ping me on chat if something here doesn't make sense, I'm only two desks away
(figuratively).

## Architecture in 10 lines

1. Scanners run in CI and on a nightly schedule and publish one message per finding.
2. Subject is `findings.<scanner>`; payload is in `contracts/findings.v1.asyncapi.yaml`.
3. Everything lands in one JetStream stream, `FINDINGS` (`deploy/stream.json`).
4. The relay owns one durable pull consumer on it, `finding-relay`.
5. It fetches 10 messages at a time and handles them one by one (`internal/relay/handler.go`).
6. Suppressed fingerprints (from the ConfigMap) are acked and skipped.
7. Fingerprints it has already ticketed are acked and skipped, so re-scans don't page twice.
8. Otherwise: create a ticket (`internal/ticket`), post to chat (`internal/chat`), ack.
9. If something fails it naks and JetStream hands the message back later.
10. A small HTTP server serves `/healthz` and the replay endpoint (`internal/admin`).

## JetStream for people who know Kafka

I came from Kafka too; this is the cheat sheet I wish I'd had.

- **Subject.** A dot-separated routing key, like `findings.gitleaks`. `*`
  matches one token, `>` matches the rest. Closer to a topic name than
  anything else, but there are no partitions.
- **Stream.** The persisted log. One stream can capture many subjects
  (`findings.>` here). Roughly a Kafka topic. It keeps messages according to
  its limits (age, count, bytes) and the discard policy; ours are in
  `deploy/stream.json`.
- **Sequence numbers.** Every message in a stream gets a stream sequence,
  which is basically the offset. A consumer also counts its own deliveries
  (consumer sequence), and that one goes up on redeliveries too.
- **Consumer.** Server-side state for reading a stream: where you are, and
  which delivered messages are still waiting for an ack. Think consumer group
  plus committed offsets, except the server tracks acks per message instead of
  one offset per partition.
- **Durable.** A named consumer that survives the client going away. Ours is
  `finding-relay`. Ephemeral consumers vanish when the client does.
- **Pull vs push.** Push: the server sends messages to a subject you listen
  on. Pull: you ask for a batch (`Fetch`). We use pull. Several clients pulling
  from the same durable share the work, a bit like group members, but any
  message can go to any puller; there's no partition assignment.
- **Ack / Nak / Term.** Ack: done, never send it again. Nak: not done, send
  it again (straight away, or after a delay if you ask). Term: give up on this
  message, never send it again. There's also InProgress, which resets the
  AckWait clock for slow work.
- **AckWait.** How long the server waits for an ack before it assumes you
  died and redelivers. Ours is 30 s.
- **MaxDeliver.** Cap on how many times one message is delivered. The default
  is unlimited.
- **Redelivery.** Anything nak'd or not acked within AckWait goes out again.
  The delivery count is in the message metadata. Redeliveries can arrive out
  of order with respect to new messages.
- **`Nats-Msg-Id` and the duplicate window.** If a publisher sets this header,
  the stream drops a second publish with the same id inside the duplicate
  window (2 minutes for us). It's dedup on the way in, at publish time. Our
  publisher sets it to the finding `id`.
- **DeliverPolicy and replay.** Where a new consumer starts: `all` (sequence
  1), `new`, `last`, `by_start_sequence`, `by_start_time`, `last_per_subject`.
  "Replay" just means a consumer starting earlier than where you are now. It
  can only replay what the stream still holds.

## Known issues

- **Occasional duplicate tickets.** Every few days someone gets two tickets
  for the same finding, a few minutes apart. It's probably the ticket API
  (their latency spikes, and I've seen it in the 5xx graphs around the same
  time). I haven't had time to dig in. The in-memory dedup catches the
  re-scan case, which was the big one.
- **In-memory dedup.** The relay runs as a single instance, so the in-memory
  dedup is fine. A map was the simplest thing that stopped the re-scan pages,
  and it has held up.
- **Replay endpoint: use with care.** It was meant to be temporary. Token is in
  env (`ADMIN_TOKEN`). I used it for the June backfill after the ticket API
  outage and it did the job. It replays the whole stream, so expect noise
  and give the ticketing folks a heads-up first. Still on my list to rotate
  the token.
- **Rate limits.** When the ticket API returns 429 we drop the finding (see
  the runbook). The scanners re-emit on the next run, so it comes back.
- **Bad messages.** If a scanner sends something that doesn't parse, the relay
  logs "bad finding, will retry" and naks it. I've meant to add alerting on
  that.
- **Dockerfile** uses `golang:latest` for both stages. Works, image is big.
  Didn't get to it.
- **nats.go.** Still on the old `nc.JetStream()` API. The new `jetstream`
  package is nicer; not urgent.

## Runbook

**Logs.** The relay logs JSON to stdout. In the cluster:

```sh
kubectl -n security logs deploy/finding-relay -f
```

Locally: `make logs`. Useful messages: `finding received`, `ticket created`,
`ticket already exists`, `ticket API rate limited, will retry`, `chat webhook
failed, will retry`, `finding breaks the contract, line ignored`, `bad
finding, dropping`, `replay requested`, `replay requested beyond stream
retention`.

**Consumer lag.** `make stream` locally, or the same `nats stream info FINDINGS`
/ `nats consumer info FINDINGS finding-relay` with your own creds against the
cluster. Look at:

- *Unprocessed Messages*: what the relay hasn't been given yet (the lag).
- *Outstanding Acks*: delivered but not acked yet. Should be at most a batch.
- *Redelivered Messages*: things that came back after a nak or AckWait. A
  small number is normal when the ticket API wobbles.

**Replay.** Safe to run: a finding that already has a ticket is skipped, with
no new ticket and no chat post. Still tell the ticketing team first, because
every replayed message costs a search call against their rate limit.

```sh
kubectl -n security port-forward deploy/finding-relay 8080:8080
curl -X POST -H "X-Admin-Token: $ADMIN_TOKEN" \
  "localhost:8080/admin/replay?since=2026-10-20T00:00:00Z"
# {"since":"...","status":"replaying","oldest_available":"..."}
```

The consumer is recreated from `since` and the relay works through the
stream again. The stream holds 7 days; a `warning` in the response means
`since` is older than that. Watch `make stream` until Unprocessed Messages is
back to zero. The endpoint is disabled when `ADMIN_TOKEN` is not set.

**Ticket API rate limiting.** The client is paced by `TICKETS_RPS` (per pod;
the platform's 5/s is per token across all pods). A 429 is retried after a
delay, never dropped. Sustained `ticket API rate limited, will retry` means
the per-pod rate times the replica count is above 5.

**Chat webhook down.** The relay naks and retries until it's back. Nothing
to do; it catches up on its own.

**Suppressing a finding.** AppSec signs off with a ticket; add the fingerprint
to `SUPPRESS_FINGERPRINTS` in `deploy/configmap.yaml` with a comment naming
the ticket, merge, and restart the deployment.

**Deploying.** See `deploy/README.md`. Anything in `deploy/` that changes
capacity or retention (replicas, the HPA, the stream limits) goes past Ravi
first; a two-line message is enough and he answers quickly. The move to two
replicas in August was his.

## Who to ask

- Ravi Menon, platform lead: the deployment, the on-call rota, capacity and
  retention (see Deploying).
- Helena Marsh, ticketing platform: the ticket API, tokens, rate limits.
- Priya Natarajan, scanner platform: the scanners, the message contract,
  who publishes what.
- Tomasz Kowal, SRE: the cluster, the NATS servers, change windows.
- Grace Adeyemi (AppSec) and Marco Lindqvist (Detection): the people who
  read the tickets and the chat channel.
- Me, for why the code is the way it is. I answer, just slower.

Good luck, and thanks for taking it. It's a nice little service; it just
never got the second pass it deserved.

Dana
