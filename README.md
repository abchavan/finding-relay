# finding-relay

Scanners (semgrep, gitleaks, osv, trivy) publish findings to the `FINDINGS`
JetStream stream on `findings.<scanner>`. `finding-relay` pulls them off a
durable consumer, opens a ticket in the ticketing platform for each new
finding, and posts a one-liner to the security chat channel. Findings whose
fingerprint is on the suppression list are acknowledged without a ticket.

The message format is in `contracts/findings.v1.asyncapi.yaml`. Notes from
the previous owner, a JetStream glossary and the runbook are in
`HANDOVER.md`.

Maintainer: Dana Okafor (security platform). Questions in #sec-platform.

## Dev loop

You need Go 1.22+ and Docker. Everything runs locally in `dev/docker-compose.yaml`:
`nats` (4222, monitoring on 8222), a mock ticket API (8081, see
`dev/ticket-api/API.md`), a mock chat sink (8082) and the relay (8080).
If you already run NATS locally, pick another host port for the container:
`NATS_PORT=14222 NATS_MONITOR_PORT=18222 make up` (every `make` target
honours `NATS_PORT`).

| Command | What it does |
| --- | --- |
| `make up` | builds and starts the stack, waits for the health checks |
| `make smoke` | `up`, resets both mocks, publishes the first 25 samples, waits for 25 tickets |
| `make publish FILE=dev/samples/findings.ndjson N=50` | publishes the first `N` lines of a file as user `scanner` to `$(PREFIX).<scanner>` (`N=0` or unset = all; `PREFIX` defaults to `findings`) |
| `make logs` | follows the relay logs (JSON, one line per event) |
| `make stream` | `nats stream info` and `nats consumer info` via a nats-box container |
| `make test` | `go test ./...` |
| `make down` | stops the stack and drops the NATS volume |

Handy while poking at it:

```sh
curl -s localhost:8081/stats      # tickets, duplicates by external_ref, 429 count
curl -s localhost:8081/tickets    # everything the mock has received
curl -s localhost:8082/messages   # everything the chat sink has received
```

## Configuration

All configuration is environment variables.

| Variable | Default | Notes |
| --- | --- | --- |
| `NATS_URL` | `nats://127.0.0.1:4222` | |
| `NATS_USER` | (empty) | |
| `NATS_PASSWORD` | (empty) | |
| `STREAM` | `FINDINGS` | created on startup if missing |
| `CONSUMER` | `finding-relay` | durable pull consumer, created on startup if missing |
| `SUBJECTS` | `findings.>` | stream subjects and consumer filter |
| `TICKETS_URL` | (empty) | base URL of the ticket API |
| `TICKETS_TOKEN` | (empty) | bearer token for the ticket API |
| `CHAT_WEBHOOK_URL` | (empty) | full webhook URL including the path token |
| `SUPPRESS_FINGERPRINTS` | (empty) | comma-separated fingerprints to ack without a ticket |
| `ADMIN_TOKEN` | (empty) | token for the admin endpoint; replay is disabled when empty |
| `HTTP_ADDR` | `:8080` | health check and admin endpoint |
| `BATCH` | `1` | messages per pull |
| `TICKETS_RPS` | `4` | requests per second to the ticket API from this pod. The platform limit (5/s) is per token, so split it across replicas |

## Admin endpoint

`GET /healthz` returns `ok`. `POST /admin/replay?since=<RFC 3339>` with an
`X-Admin-Token` header redelivers everything published since that time.
Findings that already have a ticket are skipped: no new ticket, no chat post.
The stream only holds 7 days; if `since` is older, the response has a
`warning` and `oldest_available`. See the runbook in `HANDOVER.md`.

## Deploy

Kubernetes manifests and the stream config are in `deploy/`; how they are
applied is in `deploy/README.md`. CI builds and pushes the image on merge to
`main`.
