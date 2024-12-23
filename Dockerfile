# ---- Stage 0 ----
# Builds media repo binaries
FROM golang:1.22-alpine3.18 AS builder

# Install build dependencies
RUN apk add --no-cache git musl-dev dos2unix build-base libde265-dev

WORKDIR /opt
COPY . /opt

# Build libheif manually
RUN apk add --no-cache build-base libtool cmake libjpeg-turbo-dev x265-dev ffmpeg-dev zlib-dev
RUN git clone https://github.com/strukturag/libheif.git
WORKDIR /opt/libheif
RUN git checkout v1.17.6
RUN mkdir build
WORKDIR /opt/libheif/build
RUN cmake --preset=release ..
RUN make
RUN make install
WORKDIR /opt

# Run remaining build steps
RUN dos2unix ./build.sh ./docker/run.sh && chmod 744 ./build.sh
RUN ./build.sh

# ---- Stage 1 ----
# Final runtime stage.
FROM alpine:3.18

RUN mkdir /plugins
RUN apk add --no-cache \
        su-exec \
        ca-certificates \
        dos2unix \
        imagemagick \
        ffmpeg

COPY --from=builder /opt/bin/plugin_antispam_ocr /plugins/
COPY --from=builder \
 /opt/bin/media_repo \
 /opt/bin/import_synapse \
 /opt/bin/import_dendrite \
 /opt/bin/export_synapse_for_import \
 /opt/bin/export_dendrite_for_import \
 /opt/bin/import_to_synapse \
 /opt/bin/gdpr_export \
 /opt/bin/gdpr_import \
 /opt/bin/s3_consistency_check \
 /opt/bin/combine_signing_keys \
 /opt/bin/generate_signing_key \
 /usr/local/bin/

COPY ./config.sample.yaml /etc/media-repo.yaml.sample
COPY ./docker/run.sh /usr/local/bin/
RUN dos2unix /usr/local/bin/run.sh && chmod 744 /usr/local/bin/run.sh

ENV REPO_CONFIG=/data/media-repo.yaml

CMD /usr/local/bin/run.sh
VOLUME ["/data", "/media"]
EXPOSE 8000
