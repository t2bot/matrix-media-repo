FROM matrixdotorg/sytest:latest

RUN apt-get -qq install -y curl nginx dos2unix \
    && curl -O https://dl.google.com/go/go1.10.2.linux-amd64.tar.gz \
    && tar xvf go1.10.2.linux-amd64.tar.gz \
    && chown -R root:root ./go \
    && mv go /usr/local \
    && mkdir /go \
    && openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout /etc/ssl/private/nginx-selfsigned.key -out /etc/ssl/certs/nginx-selfsigned.crt -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com" \
    && openssl dhparam -out /etc/ssl/certs/dhparam.pem 2048

COPY docker/sytest/site.conf /etc/nginx/sites-available/default
COPY docker/sytest/run-tests.sh /test/run-tests.sh
COPY docker/sytest/media-repo.yaml /test/media-repo.yaml
COPY docker/sytest/03ascii.patch /test/03ascii.patch

RUN dos2unix /test/run-tests.sh

ENV GOPATH=/go
ENV PATH="${PATH}:/usr/local/go/bin:/go/bin"