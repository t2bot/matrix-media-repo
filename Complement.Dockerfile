# ---- Stage 0 ----
# Builds media repo binaries
FROM golang:1.14-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git musl-dev dos2unix build-base

WORKDIR /opt
COPY . /opt
RUN dos2unix ./build.sh
RUN ./build.sh

# ---- Stage 1 ----
# Final runtime stage.
FROM alpine

COPY --from=builder /opt/bin/media_repo /opt/bin/complement_hs /usr/local/bin/

RUN apk add --no-cache ca-certificates postgresql openssl dos2unix

RUN mkdir -p /data/media
COPY ./docker/complement.yaml /data/media-repo.yaml
ENV REPO_CONFIG=/data/media-repo.yaml
ENV SERVER_NAME=localhost
ENV PGDATA=/data/pgdata
ENV MEDIA_REPO_UNSAFE_FEDERATION=true

COPY ./docker/complement.sh ./docker/complement-run.sh /usr/local/bin/
RUN dos2unix /usr/local/bin/complement.sh /usr/local/bin/complement-run.sh

EXPOSE 8008
EXPOSE 8448

RUN chmod +x /usr/local/bin/complement.sh
RUN chmod +x /usr/local/bin/complement-run.sh

RUN mkdir -p /data/pgdata
RUN mkdir -p /run/postgresql
RUN chown postgres:postgres /data/pgdata
RUN chown postgres:postgres /run/postgresql
RUN su postgres -c initdb
RUN sh /usr/local/bin/complement.sh

CMD /usr/local/bin/complement-run.sh