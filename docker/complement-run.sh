#!/usr/bin/env sh
openssl req -new -newkey rsa:1024 -days 365 -nodes -x509 -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=${SERVER_NAME}" -keyout /data/server.key  -out /data/server.crt
sed -i "s/SERVER_NAME/${SERVER_NAME}/g" /data/media-repo.yaml
su postgres -c "postgres -h 0.0.0.0" &
sleep 12
/usr/local/bin/media_repo &
/usr/local/bin/complement_hs
