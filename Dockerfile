# ---- Stage 0 ----
# Builds media_repo and import_synapse
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git musl-dev \
 && go get github.com/constabulary/gb/...

WORKDIR /opt

COPY ./vendor /opt/vendor
COPY ./src /opt/src
RUN GOPATH=`pwd`/vendor gb vendor restore

RUN GOPATH=`pwd`:`pwd`/vendor go build -v -o /opt/bin/media_repo ./src/github.com/turt2live/matrix-media-repo/cmd/media_repo/ \
 && GOPATH=`pwd`:`pwd`/vendor go build -v -o /opt/bin/import_synapse ./src/github.com/turt2live/matrix-media-repo/cmd/import_synapse/

# ---- Stage 1 ----
# Final runtime stage.
FROM alpine

RUN apk add --no-cache \
        su-exec \
        ca-certificates

COPY --from=builder /opt/bin/media_repo /opt/bin/import_synapse /usr/local/bin/

COPY ./config.sample.yaml /etc/media-repo.yaml.sample
COPY ./migrations /var/lib/media-repo-migrations
COPY ./docker/run.sh /usr/local/bin/

CMD /usr/local/bin/run.sh
VOLUME ["/data", "/media"]
EXPOSE 8000
