# Deploying finding-relay

The relay runs in the `security` namespace of the production cluster.

## Image

CI builds the image from the root `Dockerfile` on every merge to `main` and
pushes `registry.northwick.example/security/finding-relay:latest`. There is no
separate release step; restarting the deployment picks up the new image.

## Manifests

```sh
kubectl apply -k deploy/
kubectl -n security rollout restart deploy/finding-relay   # if only the image changed
kubectl -n security rollout status deploy/finding-relay
```

`kustomization.yaml` pulls in the deployment, the HPA, the ConfigMap and the
Secret. Configuration changes (suppression list, URLs) go in
`configmap.yaml`; the relay reads them at startup, so restart after applying.

## Secrets

Secrets are sealed in the real cluster; the committed `secret.yaml` is the dev
copy. If you rotate something, update the sealed secret through the platform
team's usual process, not this file.

## Stream

The relay creates the stream on startup if it does not exist, but the stream
in the cluster is managed from `stream.json`:

```sh
nats stream add --config deploy/stream.json      # first time
nats stream edit FINDINGS --config deploy/stream.json   # after a change
nats stream info FINDINGS
```

Use your own NATS credentials for this, not the relay's.
