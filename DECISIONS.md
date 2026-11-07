# Decisions

**Start:** 2026-11-07 06:02 UTC  **End:** 2026-11-07 07:07 UTC  **Overrun:** none

My production background is Java/Spring Boot and Kafka; this was my first
real Go codebase, so I kept changes small and leaned on tests. I used an AI
assistant; its session logs are attached as the brief asks.

## Problems

| # | Problem | Status |
|---|---|---|
| 1 | 429 → `Term()`: findings dropped (47 on 15 Oct, 5 critical) | **Fixed.** Nak with delay; client paced by `TICKETS_RPS`. |
| 2 | Duplicates: per-pod memory dedup, chat failure re-creates, timeout retry | **Fixed.** Look up by `external_ref` before create; owed chat remembered; creates 15 s, retry after 30 s. |
| 3 | Replay floods; "since 1 March" beyond 7-day retention; token `changeme` | **Fixed** (replay `?since=`, skips ticketed findings, warns beyond retention, off without token). Retention **escalated** to Tomasz/Ravi (OUTBOX 2, 3). |
| 4 | No client rate limit; HPA can scale past the shared 5 rps | **Fixed** in the relay (2 rps/pod). Replicas/HPA **escalated** to Ravi (OUTBOX 3). |
| 5 | Leaked secrets copied to chat, logs, tickets | **Fixed**: gitleaks match masked; log fields chosen, not the whole finding. Full value in tickets: asked Grace (OUTBOX 6). |
| 6 | Unparseable messages retried forever | **Fixed**: bad `line` tolerated (finding kept); real garbage `Term`'d with an error log. |
| 7 | Prod chat webhook URL committed in `deploy/secret.yaml` | **Escalated** (OUTBOX 2). Replaced in the file; rotation needed, as it is in history. |
| 8 | Contract v2: new fingerprint re-tickets every open finding | **Fixed for 2 Nov** (`CONTRACT_VERSION`). 16 Nov **escalated** to Priya (OUTBOX 9). |
| 9 | Dockerfile, nats.go API, metrics | **Left.** Image size and slow builds only; metrics are the next thing I'd add (INCIDENT.md, "What should have alerted"). |

**Left, with the risk I accept:**
- Two pods handling the same fingerprint in the same second can both miss in
  search and both create. Two emissions within a second is rare; closing it
  needs create-if-absent on the ticket API (asked Helena).
- If a create errors after succeeding and the retry lands on the other pod,
  the ticket exists but chat is not posted (19 of 243 in the 2-relay chaos run from the preparation session; a later replay posted 10 of them, with no repeats).
  The ticket is never lost.
- Each new finding costs two API calls, so throughput is about 1 new finding/s
  per pod. A 7-day replay (~2,000 messages) takes 10-15 minutes.
- Rolling a `CONTRACT_VERSION` change: old and new pods each ack the other
  version for a minute. Runbook: switch during dual-publish, then replay
  from 30 minutes before the switch.

**Duplicates vs drops (Grace and Marco):** I didn't trade one for the
other. Every residual above is a missed chat post or a rare duplicate, never
a missed ticket.

## Replies to the inbox

**Helena.** Thanks. The relay now stays under your limit: 2 rps per pod, 4 in
total today. A 429 is retried after a delay, not dropped. I'm not using
`Idempotency-Key`; the relay looks tickets up by `external_ref` before
creating, which is what you suggested. API.md doesn't document that search,
so I assumed `GET /tickets?external_ref=` (please confirm). Searches roughly
double our call count, still within the limit. Questions on create-if-absent
and on the `info` severity are in OUTBOX 1 and 7.

**Tomasz.** Replay is safe for Tuesday: findings that already have a ticket
get no new ticket and no chat ping, it takes a start time, and it is rate
limited. But the stream keeps 7 days, so "since 1 March" can only reach
about 20 Oct; the response says so. Anything older needs a scanner re-run or
a list from ticketing. The endpoint needs a real `ADMIN_TOKEN` in the sealed
secret before Tuesday. Monday pairing: yes please.

**Grace, Marco (Ravi).** Both are fixed without giving up the other. Drops
came from the rate limit (likely the August miss too); duplicates came from
the relay itself: per-pod memory, retries after a timeout, and a chat
failure re-creating the ticket. The relay now asks the ticket system before
creating and never drops on rate limits. Leaked keys are masked in chat. One
urgent item: 5 critical findings from 15 Oct may never have been ticketed
(OUTBOX 6).

**Priya (reply to the fourth message).** No separate stream needed.
*2 November:* the relay stays on contract 1. It tickets v1 and acks
`findings.v2.*` without a ticket, so dual-publish creates nothing twice
(tested with the preview sample). *16 November:* before v1 stops, the relay
switches to v2 (`CONTRACT_VERSION=2`, then a replay from just before the
switch). It maps integer severity to names and links `match_ref` in the
ticket. It matches existing tickets and the suppression list through
`legacy_fingerprint`. *What I ask for:* `legacy_fingerprint` on every v2
message until at least 31 Dec, or a one-off v1→v2 mapping export, or v1 kept
until one exists. Without it, the switch re-tickets every open finding
(tested: 20 of 20). Also: evidence readable by AppSec and repo owners, kept
while a ticket is open; a masked preview for secrets; and `line` kept an
integer. Answer by 28 Oct, please (OUTBOX 9).

## Verified, and how

Two places, kept apart so it's clear what ran where.

**In my recorded working session** (`make up` stack: one relay container,
mock ticket API, chat sink; see `logs/terminal/`):

- `go vet`, `go test -race ./...` and `gofmt -l .`: clean after every step.
  New tests cover lookup-before-create, a second instance, chat failure,
  failure after create, 429, bad `line`, masking, replay token/`since`/
  retention, v1/v2 gating, legacy matching, and a ticket lost on the platform.
- 300 samples with the ticket API returning 503 on every 10th create (after
  creating): 243 of 243 fingerprints ticketed, 0 duplicates, 0 429s.
- Replay since 1 March: 0 new tickets, retention warning returned.
- Contract 1 with the v2 preview dual-published: 20 tickets, not 40.
- `make smoke`: passes.
- Replay after ticketing lost every ticket (mock reset to 0): 243 of 243
  restored, no duplicates. Found during this verification: the relay had an
  in-memory "done" cache that acked findings it had seen before without asking
  the ticket API, so a replay could not restore lost tickets. Removed in "Ask
  the ticket API on every delivery"; every delivery now does one search.

**In the preparation session with the assistant** (a claude.ai chat, not attached; see `logs/README.md`.
`nats-server` 2.10 built from source, mocks and relays run as processes):

- Two relays, ticket API 503 on every 10th create (after creating), chat 500
  on every 15th post. Original code: 27 of 243 fingerprints ticketed, 264
  dropped on 429, 3 duplicates. Changed code: 243 of 243, 0 duplicates, 0 429s;
  19 chat posts missed, of which a later replay posted 10, with no repeats.
- v2 switch: contract 2 without `legacy_fingerprint` opened 20 new tickets for
  20 already-ticketed findings; with it, 0.
- The same 25-finding path as `make smoke`, at `TICKETS_RPS=4`: 25 tickets in
  about 16 s.
- INCIDENT.md and OUTBOX 6-7: every figure computed from the files in
  `incident/` with scripts in that session.

**Not verified:** the real ticket API's search endpoint, `info` handling, and
rate accounting; Kubernetes manifests (not applied); behaviour with more than
2 pods; the prod image build.

**Assumptions:** search shape as above; searches count toward 5 rps; replicas
stay at 2; `legacy_fingerprint` is the field name; prod runs this code.
