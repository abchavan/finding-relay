From: Helena Marsh <helena.marsh@northwick.example> (Ticketing Platform, team lead)
To: finding-relay owner <sec-platform@northwick.example>
Date: Mon, 19 Oct 2026 10:12 +0000
Subject: finding-relay and the ticket API: rate limit, Idempotency-Key

Hi,

Ravi said the relay has a new owner, so a quick note before you settle in.

We rate-limit at 5 requests per second per token. The relay's token has been
tripping that most nights around the scanner runs, and the 429s show up on our
dashboards as one of our noisiest integrations. Please keep your client under
the limit rather than relying on us to absorb bursts.

Second, a heads-up on the roadmap: the new ticket API ships next quarter, and
`Idempotency-Key` is being retired with it. We found very few integrations
used it correctly and it complicates our storage migration. The current API
still accepts it for now, but please don't take a new dependency on it. If you
need to check whether a ticket already exists, use the search endpoint
(filter on `external_ref`).

Happy to talk through anything else on the integration.

Thanks,
Helena
