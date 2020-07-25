# matrix-media-repo

[![Build status](https://badge.buildkite.com/4205079064098cf0abf5179ea4784f1c9113e875b8fcbde1a2.svg)](https://buildkite.com/t2bot/matrix-media-repo)

matrix-media-repo is a highly customizable multi-domain media repository for Matrix. Intended for medium to large environments
consisting of several homeservers, this media repo de-duplicates media (including remote media) while being fully compliant
with the specification. 

Smaller/individual homeservers can still make use of this project's features, though it may be difficult to set up or have 
higher than expected resource consumption - please do your research before deploying this as this project may not be useful
for your environment.

For help and support, visit [#mediarepo:t2bot.io](https://matrix.to/#/#mediarepo:t2bot.io).

## Installing / building

**Note**: developing on matrix-media-repo requires you to follow these steps at least once.

Assuming Go 1.14+ is already installed on your PATH:
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

## Deployment

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

## Importing media from synapse

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
    Usage of import_synapse:
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

## Export and import user data

The admin API for this is specified in [docs/admin.md](./docs/admin.md), though they can be difficult to use for scripts.
The `bin/gdpr_export` and `bin/gdpr_import` binaries do the process for you, and do so in memory but against the real
media repo database and datastores - this moves the resource intensiveness to the binary you're running instead of the
media repo instance, but still makes reads and writes to your database and datastores. For example, when exporting a 
user's data the binary will pull all the data locally and write it to disk for you, but during that process the user's
export is accessible via the main media repo too. The export is deleted if the binary is successful at exporting the 
data.

**Note**: Imports done through this method can affect other homeservers! For example, a user's data export could contain
an entry for a homeserver other than their own, which the media repo will happily import. Always validate the manifest
of an import before running it!

Ensuring you have your media repo config available, here's the help for each binary:

```
Usage of gdpr_export:
  -config string
        The path to the configuration (default "media-repo.yaml")
  -destination string
        The directory for where export files should be placed (default "./gdpr-data")
  -entity string
        The user ID or server name to export
  -migrations string
        The absolute path for the migrations folder (default "./migrations")
  -templates string
        The absolute path for the templates folder (default "./templates")
```

```
Usage of gdpr_import:
  -config string
        The path to the configuration (default "media-repo.yaml")
  -directory string
        The directory for where the entity's exported files are (default "./gdpr-data")
  -migrations string
        The absolute path for the migrations folder (default "./migrations")
```
