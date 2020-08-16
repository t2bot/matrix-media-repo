# Note: this builds a minimal conduit image from the git repo. It has nothing to do with the media repo directly.
# This should not be used in production. It is adapted from the official version.

FROM alpine:3.12 as builder

RUN sed -i -e 's|v3\.12|edge|' /etc/apk/repositories
RUN apk add --no-cache cargo openssl-dev git
RUN git clone https://git.koesters.xyz/timo/conduit.git /conduit
WORKDIR /conduit
RUN cargo install --path .

# --- ---------------------- ---

FROM alpine:3.12

# Non-standard port
EXPOSE 8004

RUN mkdir -p /srv/conduit/.local/share/conduit
COPY --from=builder /root/.cargo/bin/conduit /srv/conduit/
RUN set -x ; \
    addgroup -Sg 82 www-data 2>/dev/null ; \
    adduser -S -D -H -h /srv/conduit -G www-data -g www-data www-data 2>/dev/null ; \
    addgroup www-data www-data 2>/dev/null && exit 0 ; exit 1
RUN chown -cR www-data:www-data /srv/conduit
RUN apk add --no-cache ca-certificates libgcc
VOLUME ["/srv/conduit/.local/share/conduit"]
USER www-data
WORKDIR /src/conduit
ENTRYPOINT ["/srv/conduit/conduit"]
