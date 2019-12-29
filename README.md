# matrix-media-repo

[![AppVeyor badge](https://ci.appveyor.com/api/projects/status/github/turt2live/matrix-media-repo?branch=master&svg=true)](https://ci.appveyor.com/project/turt2live/matrix-media-repo)
[![CircleCI](https://circleci.com/gh/turt2live/matrix-media-repo/tree/master.svg?style=svg)](https://circleci.com/gh/turt2live/matrix-media-repo/tree/master)

matrix-media-repo is a highly customizable multi-domain media repository for Matrix. Intended for medium to large environments
consisting of several homeservers, this media repo de-duplicates media (including remote media) while being fully compliant
with the specification. 

Smaller/individual homeservers can still make use of this project's features, though it may be difficult to set up or have 
higher than expected resource consumption - please do your research before deploying this as this project may not be useful
for your environment.

For help and support, visit [#mediarepo:t2bot.io](https://matrix.to/#/#mediarepo:t2bot.io).

# Installing / building

Assuming Go 1.12+ is already installed on your PATH:
```bash
# Get it
git clone https://github.com/turt2live/matrix-media-repo
cd matrix-media-repo

# Build it
./build.sh

# Edit media-repo.yaml with your favourite editor
cp config.sample.yaml media-repo.yaml
vi /etc/matrix-media-repo/media-repo.yaml

# Run it
bin/media_repo
```

Another option is to use a Docker container (this script might need to be modified for your environment):
```bash
# Create a path for the Docker volume
mkdir -p /etc/matrix-media-repo

# Using config.sample.yaml as a template, edit media-repo.yaml with your favourite editor
vi /etc/matrix-media-repo/media-repo.yaml

docker run --rm -it -p 8000:8000 -v /etc/matrix-media-repo:/data turt2live/matrix-media-repo
```

Note that using `latest` is dangerous - it is effectively the development branch for this project. Instead,
prefer to use one of the tagged versions and update regularly.

# Deployment

This is intended to run behind a load balancer and beside your homeserver deployments. Assuming your load balancer handles SSL termination, a sample nginx config would be:

```ini
# Federation / Client-server API
# Both need to be reverse proxied, so if your federation and client-server API endpoints are on
# different `server` blocks, you will need to configure that.
server {
  listen 443 ssl;
  listen [::]:443 ssl;

  # SSL options not shown - ensure the certificates are valid for your homeserver deployment.

  # Redirect all traffic by default to the homeserver
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
```

Your synapse listener configuration would look something like this:
```yaml
listeners:
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

After importing your media, setting `enable_media_repo: false` in your Synapse configuration will disable the media repository.

# Importing media from synapse

Media is imported by connecting to your synapse database and downloading all the content from the homeserver. This is so 
you have a backup of the media repository still with synapse. **Do not point traffic at the media repo until after the 
import is complete.**

**Note**: the database options provided on the command line are for the Synapse database. The media repo will use the 
connection string in the media-repo.yaml config when trying to store the Synapse media.

**Note**: the import script is not available to the Docker container. Binaries of the script are included with every
release though if you want to avoid building it yourself.

1. Build the media repo (as stated above)
2. Edit/setup `media-repo.yaml` per the install instructions above
3. Run `bin/import_synapse`. The usage is below. 
    ```
    Usage of import_synapse.exe:
      -baseUrl string
            The base URL to access your homeserver with (default "http://localhost:8008")
      -config string
            The path to the media repo configuration (with the database section completed) (default "media-repo.yaml")
      -dbHost string
            The PostgresSQL hostname for your Synapse database (default "localhost")
      -dbName string
            The name of your Synapse database (default "synapse")
      -dbPassword string
            The password for your Synapse's PostgreSQL database. Can be omitted to be prompted when run
      -dbPort int
            The port for your Synapse's PostgreSQL database (default 5432)
      -dbUsername string
            The username for your Synapse's PostgreSQL database (default "synapse")
      -migrations string
            The absolute path the media repo's migrations folder (default "./migrations")
      -serverName string
            The name of your homeserver (eg: matrix.org) (default "localhost")
      -workers int
            The number of workers to use when downloading media. Using multiple workers risks deduplication not working as efficiently. (default 1)
    ```
    Assuming the media repository, postgres database, and synapse are all on the same host, the command to run would look something like: `bin/import_synapse -serverName myserver.com -dbUsername my_database_user -dbName synapse`
4. Wait for the import to complete. The script will automatically deduplicate media.
5. Point traffic to the media repository.
