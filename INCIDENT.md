# Incident 2026-10-15: duplicate tickets during the nightly run (relay write-up)

All times UTC. `acc` = `incident/ticket-api-access.log`, `rel` = `relay.log`,
`dup` = `ticket-duplicates.csv`, `ci` = `consumer-info.txt`, `thr` = `oncall-thread.md`.

## What happened

1. **Duplicates were there before the deploy.** `dup` has 194 rows. 146 of
   them repeat a ticket from an earlier day (gap ≥ 1 h), and 50 of those land
   between 02:55 and 03:02, before ticketing's 03:02 deploy (first row:
   `dup` NWK-48214 at 02:55:04.323Z, prior NWK-44233 on 3 Oct). That is the
   relay's dedup: an in-memory map per pod, empty after the 13 Oct drain, and
   not shared between the two pods.
2. **The 48 same-night pairs are the relay retrying a slow create.** From
   03:05:41 creates take 6-14 s (`acc` 03:05:41.796Z, 6233 ms; slowest
   03:24:45.752Z, 13945 ms). The relay's HTTP client gave up at 5 s and the
   handler nak'd for immediate redelivery, while the platform finished the
   create anyway. All 48 short-gap rows (`dup`, gap 0-20 s, 03:06:03 to
   03:24:59) are the **same finding id** delivered again (matched in `rel`),
   and in all 48 the first request ran past 5 s. Example: fingerprint
   43dfcca9… got four tickets, NWK-48296/48310/48317/48318 (`acc`
   03:05:41.796Z to 03:06:18.843Z), from one message (`rel`, id 236cf6dc).
3. **The deploy did not create duplicates.** The relay never calls search:
   `acc` holds only `POST /tickets`. The deploy's only visible effect is
   three 503s for one finding at 03:02:34.955-.985Z, retried at once and
   created once (NWK-48289, 03:02:35.000Z).
4. **Duplicates stopped when the run stopped, not at the rollback.** The last
   delivery to the relay was 03:24:55 (`rel` last line; `ci` 03:25:00 "Last
   delivery 4.88s ago"), the last duplicate is 03:24:59.697Z (`dup`, the tail
   of the 13.9 s create above), and the rollback finished at 03:26:30 (`thr`
   03:27). The slow window (03:05-03:25 in `acc`) matches Jonas's storage
   compaction.
5. **47 findings were dropped, 5 of them critical.** On 429 the relay
   terminated the message: 52 `ticket API rate limited, dropping` lines in
   `rel` (02:58 and 03:24). 47 of those fingerprints never got a ticket in the
   window, including live Stripe keys in `northwick/payments-api` and
   `northwick/merchant-portal` and two keys in `northwick/risk-engine`
   (`rel` 03:24:55.122-.167). This is worse than the duplicates and was not in
   the ticket the incident was closed on.
6. **Scaling to 4 made it worse.** The backlog was 12 messages (`ci` 03:10).
   The two new pods (`kube-events`, first log `rel` 03:14:44) raised the
   request rate on a 5 rps token. At 03:24 there were 32 429s in one minute
   (`acc`), 21 of them dropped by new pod q8z1m. The HPA (min 2) then scaled
   back down on its own (`kube-events`, "All metrics below target").

## Does the on-call conclusion stand?

No. "Ticketing deploy regression" does not fit the evidence: duplicates
started before the deploy, the relay does not use the service that was
changed, and they ended before the rollback did. Ticketing's account (slow,
not wrong) is right. The cause is the relay: it retries a create that is not
idempotent after a timeout shorter than the API's latency, and it dedups in
per-pod memory. The rollback was coincidence.

## What changes in the relay (commits on this branch)

- Look the ticket up by `external_ref` before creating, so redelivery,
  another pod or a restart finds the existing ticket (fixes 1 and 2).
- Creates get 15 s; failures retry after 30 s, past the slowest create seen (2).
- 429 → nak with delay, never terminate; client paced at `TICKETS_RPS` per pod (5, 6).
- Create and lookup failures are logged (they were silent: `_ = m.Nak()`).
- Runbook: scaling the relay does not drain faster; the API is the limit.

## What should have alerted

- Any `dropping` log, and a page on high/critical. Five criticals went unseen.
- Ticket create errors and timeouts from the relay. There were no logs at all.
- `num_redelivered` / ack pending on the consumer above a batch.
- Ticket API create latency above the relay's timeout.
- Ticketing's duplicate alert fired, but only on short gaps. The 146
  cross-day duplicates every night went unalerted.

## What the packet does not let me decide

- Whether the 47 dropped findings were ticketed after 03:50. The 15 Oct
  messages are past the stream's 7 days, so replay cannot recover them.
  Needs a search by fingerprint from ticketing (list sent, OUTBOX #7).
- When `kube-events.txt` was captured: its times are relative.
- Which build ran. The pulled image is 9.2 MB (`kube-events`); this repo's
  Dockerfile ships the full golang image, so prod is not built the way the
  repo says, or not from this Dockerfile.
