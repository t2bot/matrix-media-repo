# ---- Stage 0 ----
# Builds media_repo and import_synapse
FROM golang:1.12-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git musl-dev

COPY . /opt

WORKDIR /opt

RUN ./build.sh

# ---- Stage 1 ----
# Final runtime stage.
FROM alpine

COPY --from=builder /opt/bin/media_repo /opt/bin/import_synapse /usr/local/bin/

RUN apk add --no-cache \
        su-exec \
        ca-certificates

COPY ./config.sample.yaml /etc/media-repo.yaml.sample
COPY ./migrations /var/lib/media-repo-migrations
COPY ./docker/run.sh /usr/local/bin/

CMD /usr/local/bin/run.sh
VOLUME ["/data", "/media"]
EXPOSE 8000
