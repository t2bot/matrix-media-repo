FROM docker.io/alpine
COPY . /tmp/src
RUN apk add --no-cache \
      su-exec \
 && apk add --no-cache \
      -t build-deps \
      go \
      git \
      musl-dev \
 && cd /tmp/src \
 && GOPATH=`pwd` go get github.com/constabulary/gb/... \
 && PATH=$PATH:`pwd`/bin gb vendor restore \
 && GOPATH=`pwd`:`pwd`/vendor go build -o bin/media_repo ./src/github.com/turt2live/matrix-media-repo/cmd/media_repo/ \
 && GOPATH=`pwd`:`pwd`/vendor go build -o bin/import_synapse ./src/github.com/turt2live/matrix-media-repo/cmd/import_synapse/ \
 && cp bin/media_repo bin/import_synapse docker/run.sh /usr/local/bin \
 && cp config.sample.yaml /etc/media-repo.yaml.sample \
 && cp -R migrations /var/lib/media-repo-migrations \
 && cd / \
 && rm -rf /tmp/* \
 && apk del build-deps

CMD /usr/local/bin/run.sh
VOLUME ["/data", "/media"]
EXPOSE 8000
