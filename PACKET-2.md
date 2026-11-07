# Second packet

Opened after `PLAN.md` is committed (`TASK.md`). From inside the
repository:

    unzip -P second-packet-7Hq2vX ../finding-relay-packet-2.zip && git add -A && git commit -m "second packet"

Commit it before you touch anything in it. It adds:

- `incident/`: logs and exports from an incident last week, with the
  platform lead's request for a write-up (`incident/README.md`).
- `inbox/04-scanner-platform-v2.md`: a contract change from the scanner
  platform team, with `contracts/findings.v2.asyncapi.yaml` and
  `dev/samples/findings.v2-preview.ndjson`, the first 20 sample findings
  re-emitted in the v2 shape. Publish the preview into your local stack
  with `make publish FILE=dev/samples/findings.v2-preview.ndjson PREFIX=findings.v2`.

Two deliverables come with it, inside your four hours:

- **`INCIDENT.md`** (at most one page): what happened on 15 October, from
  the evidence in `incident/`, citing the lines (file and timestamp) that
  support each claim; whether the on-call conclusion stands; what changes
  in the relay; what should have alerted. If the packet does not let you
  decide something, say what is missing.
- **The reply to the fourth message**, in `DECISIONS.md` with the other
  replies: what the relay does on 2 November and on 16 November, and what
  you ask the scanner platform team for.

Priya and the people in the incident thread go to `OUTBOX.md` like
everyone else. If your plan changes, say what changed and why in `PLAN.md`.
