#ops-oncall, exported 2026-10-15

**Tomasz Kowal** (SRE, on-call) — Thu 15 Oct 2026, 03:08

ticketing's duplicate-rate alert fired for the finding-relay token. looking

**Tomasz Kowal** — 03:10

relay is up, both pods healthy, consumer has a backlog (12 unprocessed at
03:10) and redeliveries climbing. ticket API looks slow from here

**Tomasz Kowal** — 03:12

scaled finding-relay to 4 to chew through the backlog

**Tomasz Kowal** — 03:19

@Jonas Reyes you deployed ticket-api at 03:02? we started getting duplicate
tickets right after and the API is slow. can you roll back

**Jonas Reyes** (Ticketing Platform, on-call) — 03:23

that deploy was a config change to the search service, not the create path.
but ok, rolling back now

**Jonas Reyes** — 03:27

rollback finished 03:26:30

**Tomasz Kowal** — 03:29

duplicates stopped, backlog draining. closing this as ticketing deploy
regression, rolled back. 48 duplicate pairs per your alert, I'll
leave the cleanup list with you

**Jonas Reyes** — 03:41

for the record: we had elevated write latency on tickets from about 03:05 to
03:25, which was a storage compaction on our side, not the deploy. the
rollback changed nothing on the create path. happy to be proven wrong but I
don't think we were the cause of the duplicates, we only got slow

**Tomasz Kowal** — 03:44

timing says otherwise 🤷 but fine, the relay has a new owner next week, let
them write it up
