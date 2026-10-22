#sec-platform, thread exported 2026-10-22

**Grace Adeyemi** (AppSec lead) — Thu 22 Oct 2026, 09:05

Welcoming the new relay owner with a request. Duplicate tickets are the
number one complaint I hear from engineering. Teams get the same finding two
or three times, close the extras as noise, and then start treating the real
ones as noise too. Two teams are muting the channel already. Duplicates are
killing developer trust in everything we send; please fix those first.

↳ **Marco Lindqvist** (Detection lead) — 09:31

Agree duplicates are annoying, but I'd rather have duplicates than drops.
Losing a Critical is not acceptable. Near-miss from August: a leaked live
payment key in `northwick/payments-api` only got a ticket because someone saw
it in the scanner UI by hand; the relay never opened one. If the fix for
duplicates makes the relay drop anything, I'll take the duplicates.

↳ **Ravi Menon** (Platform lead) — 10:02

Both fair. The new owner starts Monday; I'll let them decide. Write up what
you choose and why, so Grace and Marco can see it.
