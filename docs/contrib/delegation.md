# Setup with host delegation

## Abstract

By default, Matrix assumes that `@alice:example.com`'s server is located at `example.com`.
The same is true Matrix Content URIs (MXC), aka. the media files.

The process of *delegating* the Synapse server to a subdomain while keeping the original server name can
be found [here](https://github.com/matrix-org/synapse/blob/master/docs/delegate.md).

This example setup uses `example.org` as server name while running Synapse and the matrix-media-repo on `matrix.example.com`.
Both services are running behind a reverse proxy.

## Configuring the media-repo

media-repo.yaml:
```yaml
repo:
  useForwaredHost: true  # See notes below
  #...

homeservers:
  - name: example.com
    csApi: https://matrix.example.com # The base URL to where the homeserver can actually be reached
    #...
```

A full sample config can be found [here](https://github.com/turt2live/matrix-media-repo/blob/master/config.sample.yaml).

The homeserver name has to match the server_name configured while also match the HTTP Host Header. If they aren't the
same, the media request gets rejected.

In scenarios the Host Header cannot be manipulated easily like with [Traefik](https://docs.traefik.io/) as
Kubernetes Ingress Controller, set `repo.useForwardedHost = true`. With this option the media-repo prefers the
`X-Forwarded-Host` over the `Host` Header as Host. Keep in mind that this might be unsuitable for your environment 
like in [#202](https://github.com/turt2live/matrix-media-repo/issues/202).

## Configuring the reverse proxy

The reverse proxy has to set the `Host` or `X-Forwarded-Host` to `example.com` as explained above.

### Traefik as Kubernetes Ingress Controller

Traefik 2.0+ is able to populate custom but **not** the original `Host` headers using a [Middleware](https://docs.traefik.io/middlewares/headers/#general).

Middleware Kubernetes Resource:
```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: matrix-host
spec:
  headers:
    customRequestHeaders:
      X-Forwarded-Host: "example.com"
```

IngressRoute Kubernetes Resource
```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: matrix
spec:
  entryPoints:
    - websecure
  routes:
  - match: Host(`matrix.example.com`) && PathPrefix(`/_matrix/`)
    kind: Rule
    services:
      - name: matrix
        port: 8008
  - match: Host(`matrix.example.com`) && PathPrefix(`/_matrix/media/`)
    kind: Rule
    services:
      - name: media-repo
        port: 8000
    middlewares:
      - name: matrix-host
```
