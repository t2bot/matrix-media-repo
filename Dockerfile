# ---- Stage 0 ----
# Builds media repo binaries
FROM golang:1.16-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git musl-dev dos2unix build-base

WORKDIR /opt
COPY . /opt
RUN dos2unix ./build.sh ./docker/run.sh
RUN ./build.sh

# ---- Stage 1 ----
# Final runtime stage.
FROM alpine

RUN mkdir /plugins
COPY --from=builder /opt/bin/plugin_antispam_ocr /plugins/
COPY --from=builder /opt/bin/media_repo /opt/bin/import_synapse /opt/bin/gdpr_export /opt/bin/gdpr_import /usr/local/bin/

RUN apk add --no-cache \
        su-exec \
        ca-certificates \
        dos2unix \
        imagemagick \
        ffmpeg

COPY ./config.sample.yaml /etc/media-repo.yaml.sample
COPY ./docker/run.sh /usr/local/bin/
RUN dos2unix /usr/local/bin/run.sh

ENV REPO_CONFIG=/data/media-repo.yaml

CMD /usr/local/bin/run.sh
VOLUME ["/data", "/media"]
EXPOSE 8000
