# Take-home: finding-relay

## Context

You have just inherited `finding-relay`, a small Go service on the security
platform team at Northwick, a fictional payments company. Scanners publish
findings to a NATS JetStream stream; the relay opens a ticket for each new
finding and posts to a chat webhook. The previous owner has moved teams. You
go on call for it on Monday. Everything in the kit is synthetic.

## How this take-home runs

Two things differ from the usual take-home. Read them before you start.

1. **A sealed second packet.** Beside the repository in the archive is
   `finding-relay-packet-2.zip`, password `second-packet-7Hq2vX`. Do not
   open it before `PLAN.md` is committed. Then, from inside the repository,
   `unzip -P second-packet-7Hq2vX ../finding-relay-packet-2.zip` and commit
   what it adds as one commit before you touch any of it. It holds new
   material from the same people and two more deliverables, a one-page
   write-up and a reply. Budget for it in `PLAN.md`; about half your time
   is a fair guess. The order is visible in your git history, your terminal
   recording and your assistant's logs. Opening it early is not a disaster;
   it is a plan written with hindsight, and we read it as one.
2. **The people are not reachable, but you write to them anyway.** When you
   would email someone from `inbox/` or `HANDOVER.md`, write the message as
   you would send it into `OUTBOX.md`, with the time. Then decide, write
   down what you assumed, and carry on. We read `OUTBOX.md` as sent mail:
   whom you asked, what, and whether you acted before or after you wrote.
   Asking is never penalised; it is often the point.

## What you receive

`finding-relay.zip`: the repository `finding-relay/` (about 1,100 lines of
Go plus fixtures) and the sealed `finding-relay-packet-2.zip` beside it.

- `README.md`, `HANDOVER.md`: the previous owner's description, a JetStream
  glossary, known issues, who to ask about what.
- `inbox/`: three short messages sent to you this week by people who depend
  on the relay.
- `cmd/`, `internal/`: the service and its existing unit tests.
- `contracts/`: the message contract.
- `deploy/`: Kubernetes manifests, stream config, credentials.
- `dev/`: `docker compose` with `nats-server`, a mock ticket API (documented
  in `dev/ticket-api/API.md`) and a mock chat sink.
- `dev/samples/`: about 300 sample findings.

`make up && make smoke` gives a working loop in under five minutes (Go
1.22+, Docker). If setup fails, note it in your terminal recording; that
time does not count.

## What we want

Make the service safe to own. You will not fix everything. Do not rewrite it.
The second packet adds two things that are due besides the code; how much of
the four hours each gets is your call, and `PLAN.md` says it.

## Deliver

1. **`PLAN.md`, committed within your first 75 minutes**: your ranked
   problems, your cut line, what you will not touch, and the time you keep
   for the second packet. Deviate later if you need to, and say why.
2. **Your changes**, as git commits on top of the kit. The second packet is
   one commit of its own, before anything in it changes.
3. **`DECISIONS.md`** (at most two pages):
   - each problem marked *fixed*, *left* (with the risk you accept) or
     *escalated* (to whom, with the message you would send);
   - a one-paragraph reply to each `inbox/` message, including the one in
     the second packet;
   - what you verified and how, what you did not verify, and your
     assumptions.
4. **`OUTBOX.md`**: every message you would have sent to a person, with the
   time and what you assumed in order to carry on. Empty is a legitimate
   answer if it is true.
5. **The two deliverables named in the second packet.**
6. **Logs** (below).

## Time cap

**4 hours** of working time, not necessarily in one sitting. Put your start
and end times at the top of `DECISIONS.md`. Stop at 4 hours, even
mid-change, and say where you stopped. If you go over, say by how much. A
declared overrun costs nothing; an undeclared one does.

## Logs

Everyone submits:

- the full git history (do not squash);
- a terminal recording (`script -T`, `asciinema rec`, or equivalent) of the
  sessions in which you worked.

If you used an AI assistant, also attach its complete, unedited session
logs. For Claude Code, attach everything a session wrote under
`~/.claude/projects/<project>/`: the `<session-id>.jsonl` file **and** the
`<session-id>/` directory next to it, which holds the transcripts of any
subagents (`subagents/`) and the full output of long tool calls
(`tool-results/`). Include every session you used, including headless
`claude -p` runs and sessions started from another directory. For any other
agent or chat tool, attach its native export, or a share link plus a copy.
Using no AI assistant is fine. You may redact personal credentials or
unrelated private content; mark each redaction `[REDACTED-BY-CANDIDATE]`. We
read the logs for decisions, not for prompt style.

## Questions

Questions about the exercise itself (setup, what counts, what we meant) by
email are answered within 4 working hours, often with "decide and
document". Do not wait for an answer: decide, record the assumption, move
on. Questions to the people in the kit go to `OUTBOX.md`; nobody answers
them during the exercise.

## Submit

One archive or a private repository link. If you advance, the next
interview builds on your submission.
