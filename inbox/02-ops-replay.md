#ops-oncall, exported 2026-10-21

**Tomasz Kowal** (SRE) — Wed 21 Oct 2026, 15:47

hey, are you the new finding-relay owner? two things

the last replay (September, someone hit /admin/replay) dumped a flood of
tickets on the ticketing side and a few hundred chat pings in #sec-findings.
took us most of an afternoon to clean up

and we need another one: the ticket API lost a batch of tickets in their
storage migration and they can't say exactly which. so next Tuesday we are
replaying everything since 1 March. please make replay safe before then, I
don't want a repeat of September

happy to pair on it Monday if useful

**Tomasz Kowal** — 15:49

(Tuesday = 27 Oct, change window is 14:00–16:00 UTC)
