# matrix-media-repo
A host-aware media repository for Matrix. Designed for environments with multiple homeservers.

Talk about it in [#media-repo:t2bot.io](https://matrix.to/#/#media-repo:t2bot.io).

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