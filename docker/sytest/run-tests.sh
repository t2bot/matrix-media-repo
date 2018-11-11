#!/bin/bash
cd /test
/go/src/github.com/turt2live/matrix-media-repo/bin/media_repo -config /test/media-repo.yaml -migrations /go/src/github.com/turt2live/matrix-media-repo/migrations & pid_mr=$!
/go/src/github.com/turt2live/matrix-media-repo/bin/sytest_homeserver & pid_hs=$!
nginx

mkdir -p /tmp/mr/uploads
mkdir -p /tmp/mr/logs

export PGDATA=/var/lib/postgresql/data
su -c '/usr/lib/postgresql/9.6/bin/initdb -E "UTF-8" --lc-collate="en_US.UTF-8" --lc-ctype="en_US.UTF-8" --username=postgres' postgres
su -c '/usr/lib/postgresql/9.6/bin/pg_ctl -w -D /var/lib/postgresql/data start' postgres
su -c 'psql -c "CREATE DATABASE mediarepo;"' postgres

patch tests/51media/03ascii.pl 03ascii.patch
./run-tests.pl -I Manual -L https://localhost --server-name example.org tests/51media/*

kill -9 $pid_mr
kill -9 $pid_hs
nginx -s stop
