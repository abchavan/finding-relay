# Outbox

Messages I would have sent, in order. Times are local working-session time.

---

## 1. To Helena Marsh (Ticketing Platform)

**Time:** 06:20
**Subject:** finding-relay: search endpoint shape, and a replay on 27 Oct

Hi Helena,

Thanks for the note. I'm changing the relay to look a ticket up by
`external_ref` before creating it, and I won't use `Idempotency-Key`.

API.md doesn't document the search endpoint. I've assumed
`GET /tickets?external_ref=<fingerprint>` returning an array of tickets, and
the relay also checks `external_ref` on each result in case the filter is
ignored. Could you confirm the path, and whether searches count toward the
5 rps? I've capped the relay at 2 rps per pod (2 pods today).

SRE plans a replay on Tue 27 Oct, 14:00-16:00 UTC. With the search check it
should only create tickets that are missing, at no more than 4 rps in total.

Last one: is there, or will there be, an atomic create-if-absent on the new
API? That would close the last small duplicate window.

Thanks

**Assumed to carry on:** the endpoint shape above, and that searches count
toward the limit.

---

## 2. To Tomasz Kowal (SRE), cc Ravi Menon

**Time:** 06:20
**Subject:** Tuesday's replay: the stream only holds 7 days

Hi Tomasz,

Replay will be safe to run on Tuesday: findings that already have a ticket
are skipped, with no new ticket and no chat ping. It takes a start time and
is rate limited.

The catch: FINDINGS has `max_age` of 7 days, so "everything since 1 March"
only reaches about 20 Oct. Anything older is no longer in the stream. To
recover March to October we need either a full scanner re-run (Priya; the
relay will dedup it safely now) or a list of lost refs from ticketing.

Two things for before Tuesday:
- The replay endpoint is now disabled unless `ADMIN_TOKEN` is set. It isn't in
  any manifest, so prod has been running with the default `changeme`. Can we
  add a real one to the sealed secret?
- deploy/secret.yaml has a production chat webhook URL committed (since May).
  It needs rotating; I've replaced it in the file, but it stays in git history.

Happy to pair on Monday.

**Assumed to carry on:** Tuesday goes ahead with `since` set to the oldest
message the stream holds. I am not touching retention.

---

## 3. To Ravi Menon (Platform)

**Time:** 06:20
**Subject:** relay replicas/HPA vs the ticket API rate limit

Hi Ravi,

The ticket API limit is 5 rps per token, shared by all relay pods. With 2
replicas I've set each pod to 2 rps. The HPA scales on CPU up to 6 pods; that
adds no throughput (we're bound by the API) and would put us over the limit.
I'd suggest pinning to 2 (min = max) or removing the HPA. I haven't changed
it; your call.

Also, as in my note to Tomasz: is extending retention past 7 days on the
table, or do we rely on re-scans for anything older?

**Assumed to carry on:** replicas stay at 2, retention stays at 7 days.

---

## 4. To Priya Natarajan (Scanner Platform)

**Time:** 06:20
**Subject:** osv sent a non-integer `line`

Hi Priya,

An osv finding on 2026-10-19 (fingerprint
8aecddcaf709f34f6039cc7dfbec70ada1325267) had `"line": "forty-two"`, which
breaks the v1 contract. The relay used to retry it forever; it now tickets it
with line 0 and logs a warning. Could you check the osv emitter?

Also: if ticketing can't say which tickets they lost, could the scanners do a
full re-run so findings older than the stream's 7 days come through again?

**Assumed to carry on:** a malformed `line` is cosmetic; the finding is real.

---

## 5. To Dana Okafor

**Time:** 06:20
**Subject:** two quick questions about the relay

Hi Dana,

1. The handover says single instance, but the deployment has run 2 replicas
   since 19 Aug. Was the in-memory dedup ever meant to cover that?
2. Where does the prod `ADMIN_TOKEN` come from? It isn't in the manifests. Was
   September's replay done with `changeme`?

**Assumed to carry on:** it was `changeme`, so I treat the token as leaked.
