# Sample findings

`findings.ndjson` is one day of scanner output, exported from the `FINDINGS`
stream on 2026-10-19: 300 messages, one JSON finding per line, in the order
they were published.

Publish it into the local stack (after `make up`):

```sh
make publish FILE=dev/samples/findings.ndjson         # all 300
make publish FILE=dev/samples/findings.ndjson N=25    # the first 25 lines
```

`make smoke` publishes the first 25 lines on its own.

The publisher sends each line to `findings.<scanner>` with `Nats-Msg-Id` set
to the finding's `id`, as the scanners do.

Expect repeats: scanners re-scan repositories several times a day, and a
re-scan re-emits a finding that is still present with the same `fingerprint`
and a new `id` and `emitted_at`.
