From: Priya Natarajan <priya.natarajan@northwick.example> (Scanner Platform, team lead)
To: finding-relay owner <sec-platform@northwick.example>
Cc: Ravi Menon <ravi.menon@northwick.example>
Date: Thu, 22 Oct 2026 14:20 +0000
Subject: findings contract v2: dual-publish from 2 Nov, v1 stops 16 Nov

Hi,

Ravi tells me the relay has a new owner from Monday, so this goes to you
rather than to Dana. Version 2 of the findings contract is ready and we would
like to start publishing it in two weeks. Draft contract and a preview sample
are attached (Ravi put them in your repository as
`contracts/findings.v2.asyncapi.yaml` and
`dev/samples/findings.v2-preview.ndjson`).

What changes:

1. `schema_version: 2` on every message.
2. `severity` becomes an integer 0–4 (info 0, low 1, medium 2, high 3,
   critical 4). Too many consumers were string-matching it.
3. `evidence.match` and `evidence.context` are gone. In their place
   `evidence.match_ref` is a URL into the evidence vault
   (`https://evidence.northwick.example/v1/findings/<id>/match`), readable
   with the scanner-platform read role, kept for 30 days. The raw match was
   leaving our pipeline in too many places.
4. `fingerprint` is now sha256 (64 hex) over
   `scanner|rule|repo|path|sha256(match)`. The v1 sha1 was over the raw
   match, which pushed secret-shaped text through our hashing pipeline and
   Legal wants it gone. We know what this means for you: the same finding
   has a different fingerprint in v2 than in v1. The preview sample is the
   first 20 findings of the v1 export you have under `dev/samples/`,
   re-emitted in v2 shape with new ids, so you can diff them line by line.
5. Subjects are `findings.v2.<scanner>`. We publish into the same `FINDINGS`
   stream; your `findings.>` filter will pick them up without a change on
   your side.

Dates: we start dual-publishing (every finding in both shapes) on Monday
2 November. v1 stops on Monday 16 November.

What I need from you by Wednesday 28 October: confirmation that the relay
handles dual-publish on 2 November, or a request to publish v2 into a
separate stream (`FINDINGS_V2`) instead. We would rather not: two retention
configs, two replay stories, and every other consumer is fine with the
shared stream. If there is something you need in the v2 payload that is not
in the draft, this is the week to ask.

Thanks,
Priya
