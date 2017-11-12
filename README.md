# matrix-media-repo

A host-aware media repository for Matrix. For development news and support, please visit [#media-repo:t2bot.io](https://matrix.to/#/#media-repo:t2bot.io).

Designed for environments with multiple homeservers, matrix-media-repo de-duplicates all media automatically, including remote content. Environments with only one homeserver can still make use of the de-duplication and performance of matrix-media-repo.

# Installing

Assuming Go 1.9 and JDK 1.8 are already installed on your PATH:
```
# Get it
git clone https://github.com/turt2live/matrix-media-repo
cd matrix-media-repo

# Build it
go get github.com/constabulary/gb/...
gb vendor restore
gb build

# Configure it (edit media-repo.yaml to meet your needs)
cp config.sample.yaml media-repo.yaml

# Run it
bin/matrix-media-repo
```

# Deployment

This is intended to run behind a load balancer and beside your homeserver deployments. A sample nginx configuration for this is:

```ini
# Redirect all matrix traffic by default to the homeserver
location /_matrix {
    proxy_read_timeout 60s;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_pass http://localhost:8008; # Point this towards your homeserver
}

# Redirect all media endpoints to the media-repo
location /_matrix/client/r0/media {
    proxy_read_timeout 60s;
    proxy_set_header Host $host; # Make sure this matches your homeserver in media-repo.yaml
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_pass http://localhost:8000; # Point this towards media-repo
}
```