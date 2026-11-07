# Plan

Budget: 4 hours. Roughly 2 hours for the code below, 2 hours for the second
packet (write-up and reply) and DECISIONS.md.

## Ranked problems

1. **Findings are dropped.** A 429 from the ticket API calls `Term()`, so the
   message is gone for good. "The scanner re-emits it" only holds if the
   issue is still present on the next run. Helena says we hit 429 most
   nights. This is my best guess for Marco's August near-miss (live payment
   key, no ticket). Losing a Critical is the worst thing this service can do.
2. **Duplicate tickets.** Dedup is an in-memory map, but the deployment has run
   2 replicas (HPA up to 6) since 2f0c25e on 19 Aug, and the map is empty after
   every restart. Two more causes are in the handler itself: a ticket created
   but whose chat post failed is nak'd and created again, and a create that
   times out on our side (5s client timeout) but succeeds on theirs is
   created again. HANDOVER blames the ticket API; it is mostly the relay.
3. **Replay is unsafe, and one is booked for Tue 27 Oct 14:00-16:00 UTC.** It
   deletes the consumer, restarts from sequence 1, and every message opens a
   ticket and pings chat again (September). Also, the stream keeps 7 days
   (`max_age` in stream.json), so "everything since 1 March" is not in the
   stream at all. The admin token defaults to `changeme` and is not set in
   any manifest.
4. **No client-side rate limit.** 5 rps per token, shared by every replica.
5. **Leaked secrets are copied again.** gitleaks evidence (live keys) goes into
   the chat channel, the relay logs (the whole finding is logged) and the
   ticket body.
6. **Poison messages loop forever.** A payload that does not parse is nak'd
   with no MaxDeliver.
7. **A prod chat webhook URL is committed** in deploy/secret.yaml (the "dev
   copy"), since May.
8. Dockerfile ships the full golang image; old nats.go API; no metrics.

## Cut line

Fix 1-6. 1 and 2 share one fix: before creating, look the ticket up by
`external_ref` on the ticket API (what Helena asked for), and retry instead of
dropping. No `Idempotency-Key` (being retired). Then 3 (replay from a start
time, refuse without a token, report when the request is older than the
stream), 4, 5, 6.

Escalate 7 and the retention gap in 3. Leave 8.

Duplicates vs drops: I will not trade one for the other. Every change has to
keep "never drop" true; where a residual risk is left, it is a duplicate or a
missed chat post, never a missed ticket.

## Will not touch

- No rewrite, no move to the `jetstream` package.
- Replicas, HPA and stream retention: Ravi's. I will ask, not change.
- The mock ticket API (owned by ticketing); I code against the search shape I
  assume and say so.
- Dockerfile, metrics, alerting: follow-ups.

## Order and time

| When | What |
| --- | --- |
| 0:00-1:00 | read, baseline run, this plan, first OUTBOX messages |
| 1:00-2:00 | fixes 1-6 with tests, config, docs |
| 2:00-3:30 | second packet: commit it as received, then its two deliverables |
| 3:30-4:00 | DECISIONS.md, OUTBOX.md, final test run |

If the second packet changes the ranking, I will say so in DECISIONS.md.
