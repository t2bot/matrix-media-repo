# matrix-media-repo

MMR is a highly configurable multi-homeserver media repository for Matrix. It is an optional component of your homeserver 
setup, and recommended only for large individual servers or hosting providers with many servers.

**If you're looking for an S3 connector, please consider using [synapse-s3-storage-provider](https://github.com/matrix-org/synapse-s3-storage-provider) instead.**

Smaller homeservers can still set this up, though may find it difficult to deploy or use. A high level of knowledge regarding
the Matrix homeserver stack is assumed.

## Documentation and support

Matrix room: [#media-repo:t2bot.io](https://matrix.to/#/#media-repo:t2bot.io)

Documentation: [docs.t2bot.io](https://docs.t2bot.io/matrix-media-repo/)

## Developers

MMR requires compiling at least once before it'll run in a development setting. See the [compilation steps](https://docs.t2bot.io/matrix-media-repo/unstable/installation/compile)
before continuing.

This project offers a development environment you can use to test against a client and homeserver.

As a first-time setup, run:

```bash
docker run --rm -it -v ./dev/synapse-db:/data -e SYNAPSE_SERVER_NAME=localhost -e SYNAPSE_REPORT_STATS=no matrixdotorg/synapse:latest generate
```

Then you can run `docker compose -f dev/docker-compose.yaml up` to always bring the service online. The homeserver will 
be behind an nginx reverse proxy which routes media requests to `http://host.docker.internal:8001`. To test accurately, 
it is recommended to add the following homeserver configuration to your media repo config:

```yaml
name: "localhost"
csApi: "http://localhost:8008" # This is exposed by the nginx container
```

Federated media requests should function normally with this setup, though the homeserver itself will be unable to federate.
For convenience, an element-web instance is also hosted at the same port from the root. 

A postgresql server is also created by the docker stack for ease of use. To use it, add the following to your configuration:

```yaml
database:
  postgres: "postgres://postgres:test1234@127.0.0.1:5432/postgres?sslmode=disable"
  pool:
    maxConnections: 10
    maxIdleConnections: 10
```

Note that the postgresql image is *insecure* and not recommended for production use. It also does not follow best practices
for database management - use at your own risk.

**Note**: Running the Go tests requires Docker, and may pollute your cached images with tons of layers. It is suggested to
clean these images up manually from time to time, or rely on an ephemeral build system instead.
