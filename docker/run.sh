#!/usr/bin/env sh
cd /data
if [ ! -f media-repo.yaml ]; then
    cp /etc/media-repo.yaml.sample media-repo.yaml
fi
chown -R ${UID}:${GID} /data

exec su-exec ${UID}:${GID} media_repo -migrations /var/lib/media-repo-migrations
