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

---

## 6. To Grace Adeyemi and Marco Lindqvist, cc Ravi Menon (urgent)

**Time:** 07:04
**Subject:** 5 critical findings from 15 Oct may have no ticket

Grace, Marco,

While writing up the 15 Oct incident I found that the relay dropped 47
findings that night on the ticket API's rate limit, and none of them got a
ticket in the window we have logs for. Five are critical:

- eeef61637fa6b343d08ed70ee25b0bc9780bea50 gitleaks, northwick/payments-api, Stripe live secret key
- ace7ea23fbebd093af6ee443ba11e40f7f20d39b gitleaks, northwick/merchant-portal, Stripe live secret key
- d131d85f83a9ab7b56e8e5a8fc46c3441c4499d4 gitleaks, northwick/risk-engine, AWS access key id
- ee39d39dfe41c2519df94aa6aa382a68307d8c88 gitleaks, northwick/risk-engine, generic API key
- 21b96e9c270d912cbde0bdc5820b31d849711125 trivy, northwick/notifications, CVE-2025-33845

The stream no longer holds 15 Oct, so a replay can't bring them back. If
they're still present, the nightly scan re-emits them and the fixed relay
tickets them. Could someone check these five by hand today? The full list of
47 is in INCIDENT.md's evidence; I've asked ticketing to search for them.

On duplicates vs drops: I haven't traded one for the other. The relay no
longer drops on rate limits, and it checks the ticket system before creating,
which removes both kinds of duplicate we saw that night. Details in
DECISIONS.md.

**Assumed to carry on:** none of the five was ticketed later.

---

## 7. To Jonas Reyes and Helena Marsh (Ticketing), cc Ravi Menon

**Time:** 07:04
**Subject:** 15 Oct: your deploy didn't cause the duplicates; two requests

Hi Jonas, Helena,

For Thursday's review: the evidence supports Jonas. The relay doesn't call
search; the duplicates began before 03:02 and ended before the rollback. They
came from the relay retrying creates that took longer than its 5 s timeout,
plus per-pod dedup. Write-up in INCIDENT.md.

Two requests:
1. Could you search for tickets created after 03:50 on 15 Oct for the 47
   fingerprints the relay dropped (list attached)? That tells us what is
   still missing.
2. API.md lists severities low/medium/high/critical, but the scanners also
   send `info` (about a sixth of findings). Does the real API accept `info`?
   If it rejects it, the relay would retry those forever.

**Assumed to carry on:** `info` is accepted, as the mock does.

---

## 8. To Ravi Menon

**Time:** 07:04
**Subject:** INCIDENT.md for Thursday, and two gaps in the packet

Hi Ravi,

INCIDENT.md is in the repo. Short version: the on-call conclusion doesn't
hold; the relay caused the duplicates, and it also dropped 47 findings that
night (5 critical, already flagged to Grace and Marco).

Two things I couldn't settle from the packet:
- kube-events.txt has relative times only. When was it captured?
- The image the pods pulled is 9.2 MB, but the repo's Dockerfile builds on
  the full golang image. How does CI actually build it? I want to be sure
  prod runs this code.

**Assumed to carry on:** prod runs the code in this repository.

---

## 9. To Priya Natarajan (Scanner Platform), cc Ravi Menon

**Time:** 07:04
**Subject:** Re: findings contract v2: dual-publish from 2 Nov, v1 stops 16 Nov

The reply is in DECISIONS.md ("Reply to Priya"); this is the message as sent.

Hi Priya,

Thanks for the early notice. The shared stream is fine: no FINDINGS_V2.

2 Nov: confirmed. The relay tickets v1 and acks v2 without a ticket, so
dual-publish creates nothing twice. I've tested this with your preview
sample.

16 Nov: I can switch the relay to v2 before v1 stops, but one thing blocks it.
Our open tickets, and AppSec's suppression list, are keyed on the v1
fingerprint. If the v2 fingerprint isn't derivable, the switch opens a second
ticket for every open finding (tested: 20 of 20 in your preview). What I need,
in order of preference:

1. `legacy_fingerprint` (the v1 value) on every v2 message, from 2 Nov until
   at least 31 Dec. The relay already reads it if present.
2. If Legal rules that out after 16 Nov: a one-off export mapping v1 to v2
   fingerprints for every finding open on 16 Nov.
3. If neither: keep v1 publishing until we have one of the above.

Two more things on the payload:
- `match_ref` needs the scanner-platform read role and lives 30 days.
  Ticket readers (AppSec and repo owners) don't have that role, and tickets
  stay open longer than 30 days. Can AppSec get read access, and can evidence
  be kept while a ticket is open? For secrets, a masked preview (first 4
  characters) would tell an engineer which key to rotate without the raw
  value.
- Please keep `line` an integer: osv sent `"line": "forty-two"` on 19 Oct.

Could you confirm by Wed 28 Oct?

**Assumed to carry on:** `legacy_fingerprint` arrives under that name. The
relay stays on v1 until it does.
