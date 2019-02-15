# matrix-media-repo

[![#mediarepo:t2bot.io](https://img.shields.io/badge/matrix-%23mediarepo:t2bot.io-brightgreen.svg)](https://matrix.to/#/#mediarepo:t2bot.io)
[![TravisCI badge](https://travis-ci.org/turt2live/matrix-media-repo.svg?branch=master)](https://travis-ci.org/turt2live/matrix-media-repo)
[![AppVeyor badge](https://ci.appveyor.com/api/projects/status/github/turt2live/matrix-media-repo?branch=master&svg=true)](https://ci.appveyor.com/project/turt2live/matrix-media-repo)
[![CircleCI](https://circleci.com/gh/turt2live/matrix-media-repo/tree/master.svg?style=svg)](https://circleci.com/gh/turt2live/matrix-media-repo/tree/master)

Designed for environments with multiple homeservers, matrix-media-repo de-duplicates all media automatically, including remote content. Environments with only one homeserver can still make use of the de-duplication and performance of matrix-media-repo.

# Installing

Assuming Go 1.9 is already installed on your PATH:
```bash
# Get it
git clone https://github.com/turt2live/matrix-media-repo
cd matrix-media-repo

# Set up the build tools
currentDir=$(pwd)
export GOPATH="$currentDir/vendor/src:$currentDir/vendor:$currentDir:"$GOPATH
go get github.com/constabulary/gb/...
export PATH=$PATH":$currentDir/vendor/bin:$currentDir/vendor/src/bin"

# Build it
gb vendor restore
gb build

# Configure it (edit media-repo.yaml to meet your needs)
cp config.sample.yaml media-repo.yaml

# Run it
bin/media_repo
```

### Installing in Alpine Linux

The steps are almost the same as above. The only difference is that `gb build` will not work, so instead use the following lines:
```bash
go build -o bin/media_repo ./src/github.com/turt2live/matrix-media-repo/cmd/media_repo/
go build -o bin/import_synapse ./src/github.com/turt2live/matrix-media-repo/cmd/import_synapse/
```

# Deployment

This is intended to run behind a load balancer and beside your homeserver deployments. A sample nginx configuration for this is:

```ini
# Client-server API
server {
  listen 80;
  listen [::]:80;
  # ssl configuration not shown

  # Redirect all matrix traffic by default to the homeserver
  location /_matrix {
      proxy_read_timeout 60s;
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $remote_addr;
      proxy_pass http://localhost:8008; # Point this towards your homeserver
  }
  
  # Redirect all media endpoints to the media-repo
  location /_matrix/media {
      proxy_read_timeout 60s;
      proxy_set_header Host $host; # Make sure this matches your homeserver in media-repo.yaml
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $remote_addr;
      proxy_pass http://localhost:8000; # Point this towards media-repo
  }
}

# Federation
# This also needs to be reverse proxied to capture the remote media fetching from other servers
server {
  listen 8448 ssl;
  listen [::]:8448 ssl;

  # These MUST match the certificates used by synapse!
  ssl_certificate /home/matrix/.synapse/your.homeserver.com.tls.cert;
  ssl_certificate_key /home/matrix/.synapse/your.homeserver.com.tls.key;
  ssl_dhparam /home/matrix/.synapse/your.homeserver.com.tls.dh;

  # Redirect all traffic by default to the homeserver
  location / {
      proxy_read_timeout 60s;
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $remote_addr;
      proxy_pass http://localhost:8008; # Point this towards your homeserver
  }
  
  # Redirect all media endpoints to the media-repo
  location /_matrix/media {
      proxy_read_timeout 60s;
      proxy_set_header Host $host; # Make sure this matches your homeserver in media-repo.yaml
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $remote_addr;
      proxy_pass http://localhost:8000; # Point this towards media-repo
  }
}
```

Your synapse listener configuration would look something like this:
```yaml
listeners:
  - port: 8558
    bind_addresses: ['127.0.0.1']
    type: http
    tls: true
    x_forwarded: true
    resources:
      - names: [federation]
        compress: false
  - port: 8008
    bind_addresses: ['127.0.0.1']
    type: http
    tls: false
    x_forwarded: true
    resources:
      - names: [client]
        compress: true
      - names: [federation]
        compress: false
```

# Importing media from synapse

Media is imported by connecting to your synapse database and downloading all the content from the homeserver. This is so you have a backup of the media repository still with synapse. **Do not point traffic at the media repo until after the import is complete.**

1. Build the media repo
2. Configure the `media-repo.yaml`
3. Run `bin/import_synapse`. The usage is below. 
    ```
    Usage of ./bin/import_synapse:
      -baseUrl string
            The base URL to access your homeserver with (default "http://localhost:8008")
      -dbHost string
            The IP or hostname of the postgresql server with the synapse database (default "localhost")
      -dbName string
            The name of the synapse database (default "synapse")
      -dbPassword string
            The password to authorize the postgres user. Can be omitted to be prompted when run
      -dbPort int
            The port to access postgres on (default 5432)
      -dbUsername string
            The username to access postgres with (default "synapse")
      -serverName string
            The name of your homeserver (eg: matrix.org) (default "localhost")
    ```
    Assuming the media repository, postgres database, and synapse are all on the same host, the command to run would look something like: `bin/import_synapse -serverName myserver.com -dbUsername my_database_user -dbName synapse`
4. Wait for the import to complete. The script will automatically deduplicate media.
5. Point traffic to the media repository
