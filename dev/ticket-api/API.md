# Ticketing API

Maintained by the ticketing platform team. Base URL in dev: `http://ticket-api:8081`.

## Authentication

Every create needs `Authorization: Bearer <token>`. Tokens are issued per
integration; a missing or empty token gets `401`.

## Endpoints

### `POST /tickets`

Creates a ticket. Request body (JSON):

| field          | type             | notes                                  |
| -------------- | ---------------- | -------------------------------------- |
| `title`        | string           | required                               |
| `body`         | string           | free text                              |
| `severity`     | string           | `low`, `medium`, `high`, `critical`    |
| `external_ref` | string           | your own reference for the ticket      |
| `labels`       | array of strings | optional                               |

Returns `201` with `{id, title, severity, external_ref, created_at}`; ids look
like `NWK-1001`.

Send `Idempotency-Key` on every create; a repeated key within 24 hours returns the existing ticket.
The repeat is answered with `200` and the original ticket instead of `201`.

### `GET /tickets`

Returns all tickets as an array.

### `GET /tickets/{id}`

Returns one ticket, or `404`.

### `GET /healthz`

Returns `200 ok`.

## Rate limit

5 requests per second per token, sustained, with bursts of up to 30. Over the limit you get a
`429` with `Retry-After: 1` and the body `{"error":"rate limited"}`. Wait and
retry.

## Dev mock only

These endpoints exist only in the dev mock and are not part of the real API.

- `GET /stats` returns `{"tickets", "unique_external_refs", "duplicates": [{"external_ref", "count"}], "rate_limited"}`.
- `POST /reset` returns `204` and clears tickets, idempotency keys, counters and chaos settings.
- `POST /chaos` with `{"fail_every": N, "status": 503, "delay_ms": D, "after_create": false}` makes every Nth create fail with `status`. With `after_create: true` the ticket is created before the error is returned. `delay_ms` adds latency to creates; `fail_every: 0` disables.
