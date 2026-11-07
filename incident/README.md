# Incident 2026-10-15: duplicate tickets during the nightly scanner run

Ravi Menon, 2026-10-21

Nobody owned the relay on the night of Thursday 15 October, so SRE on-call
handled it. Short version as ops closed it: between 03:00 and 03:30 UTC the
relay opened duplicate tickets (48 pairs by ticketing's count),
on-call scaled the relay up to work through the backlog, asked the ticketing
platform to roll back a deploy they had made at 03:02, and the duplicates
stopped a minute after the rollback. Closed as "ticketing deploy regression,
rolled back".

The ticketing platform disputes that. Jonas Reyes (their on-call) says the
03:02 deploy was a configuration change to their search service, the create
path was not touched, and they had elevated write latency between roughly
03:05 and 03:25 from a storage job. Both versions are in the thread below.

The incident review is on Thursday 29 October and both teams will be in the
room. I need the relay's write-up before then: `INCIDENT.md` in this
repository, one page. What happened, from the evidence; whether the on-call
conclusion stands; what changes in the relay; what we should have been
alerted on. If you need something that is not in this directory, ask me and
I will chase it.

## What is here

| File | Source |
|---|---|
| `oncall-thread.md` | `#ops-oncall`, exported by Tomasz Kowal, 03:08–03:44 UTC |
| `relay.log` | `kubectl -n security logs --prefix --timestamps` for every relay pod, 02:50–03:50 UTC, merged and sorted by time (Tomasz) |
| `ticket-api-access.log` | the ticketing platform's access log for the relay's token, 02:50–03:50 UTC (Jonas) |
| `ticket-duplicates.csv` | ticketing's duplicate report: every ticket created 02:50–04:00 UTC whose `external_ref` already had a ticket, with the prior ticket (Jonas) |
| `consumer-info.txt` | `nats consumer info FINDINGS finding-relay`, as captured every five minutes by the ops monitoring job, 03:00–03:40 UTC (Tomasz) |
| `kube-events.txt` | `kubectl -n security get events` for the window (Tomasz) |

Times are UTC everywhere. The relay deployment was not changed that night;
the two pods had been running since the node drain on 13 October.
