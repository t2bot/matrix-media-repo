# Media repository administration

All the API calls here require your user ID to be listed in the configuration as an administrator. After that, your access token for your homeserver will grant you access to these APIs. The URLs should be hit against a configured homeserver. For example, if you have `t2bot.io` configured as a homeserver, then the admin API can be used at `https://t2bot.io/_matrix/media/unstable/admin/...`.

## Media purge

Sometimes you just want your disk space back - purging media is the best way to do that. **Be careful about what you're purging.** The media repo will happily purge a local media object, making it highly unlikely to ever exist in Matrix again. When the media repo deletes remote media, it is only deleting its copy of it - it cannot delete media on the remote server itself. Thumbnails will also be deleted for the media.

#### Purge remote media

URL: `POST /_matrix/media/unstable/admin/purge/remote?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete remote media from the file store that was downloaded before the timestamp specified. If the file is referenced by newer remote media or local files to any of the configured homeservers, it will not be deleted. Be aware that removing a homeserver from the config will cause it to be considered a remote server, and therefore the media may be deleted.

Any remote media that is deleted and requested by a user will be downloaded again.

#### Purge quarantined media

URL: `POST /_matrix/media/unstable/admin/purge/quarantined?access_token=your_access_token`

This will delete all media that has previously been quarantined, local or remote.

#### Purge individual record

URL: `POST /_matrix/media/unstable/admin/purge/<server>/<media id>?access_token=your_access_token`

This will delete the media record, regardless of it being local or remote.

## Quarantine media

URL: `POST /_matrix/media/unstable/admin/quarantine/<server>/<media id>?access_token=your_access_token`

The `<server>` and `<media id>` can be retrieved from an MXC URI (`mxc://<server>/<media id>`).

The quarantine media API allows administrators to quarantine media that may not be appropriate for their server. Using this API will prevent the media from being downloaded any further. It will *not* delete the file from your storage though: that is a task left for the administrator.

Remote media that has been quarantined will not be purged either. This is so that the media remains flagged as quarantined. It is safe to delete the file on your disk, but not delete the media from the database.

Quarantining media will also quarantine any media with the same file hash.

This API is unique in that it can allow administrators of configured homeservers to quarantine media on their homeserver only. This will not allow local administrators to quarantine remote media or media on other homeservers though, just on theirs.

## Datastore management

Datastores are used by the media repository to put files. Typically these match what is configured in the config file, such as s3 and directories. 

#### Listing available datastores

URL: `GET /_matrix/media/unstable/admin/datastores?access_token=your_access_token`

The result will be something like: 
```json
{
  "00be9363007feb66de554a79e16b7b49": {
    "type": "file",
    "uri": "/mnt/media"
  },
  "2e17bad1bf76c9618e3cde30166dc674": {
    "type": "s3",
    "uri": "s3:\/\/example.org\/bucket-name"
  }
}
```

In the above response, `00be9363007feb66de554a79e16b7b49` and `2e17bad1bf76c9618e3cde30166dc674` are datastore IDs.

#### Estimating size of a datastore

URL: `GET /_matrix/media/unstable/admin/datastores/<datastore id>/size_estimate?access_token=your_access_token`

Sample response:
```json
{
  "thumbnails_affected": 672,
  "thumbnail_hashes_affected": 598,
  "thumbnail_bytes": 49087657,
  "media_affected": 372,
  "media_hashes_affected": 346,
  "media_bytes": 340907359,
  "total_hashes_affected": 779,
  "total_bytes": 366601489
}
```

#### Transferring media between datastores

URL: `POST /_matrix/media/unstable/admin/datastores/<source datastore id>/transfer_to/<destination datastore id>?access_token=your_access_token`

The response is the estimated amount of data being transferred:
```json
{
  "task_id": 12,
  "thumbnails_affected": 672,
  "thumbnail_hashes_affected": 598,
  "thumbnail_bytes": 49087657,
  "media_affected": 372,
  "media_hashes_affected": 346,
  "media_bytes": 340907359,
  "total_hashes_affected": 779,
  "total_bytes": 366601489
}
```

The `task_id` can be given to the Background Tasks API described below.

## Data usage for servers/users

Individual servers and users can often hoard data in the media repository. These endpoints will tell you how much. These endpoints can only be called by repository admins - they are not available to admins of the homeservers.

**Caution**: These endpoints may return *lots* of data. Making very specific requests is recommended.

#### Per-server usage

URL: `GET /_matrix/media/unstable/admin/usage/<server name>?access_token=your_access_token`

The response is how much data the server is using:
```json
{
  "raw_bytes": {
    "total": 1594009,
    "media": 1392009,
    "thumbnails": 202000
  },
  "raw_counts": {
    "total": 7,
    "media": 4,
    "thumbnails": 3
  }
}
```

**Note**: The endpoint may return values which represent duplicated media across itself and other hosts.

#### Per-user usage (all known users)

URL: `GET /_matrix/media/unstable/admin/usage/<server name>/users?access_token=your_access_token`

The response is how much data the server is using:
```json
{
  "@alice:example.org": {
    "raw_bytes": {
      "total": 1392009,
      "media": 1392009
    },
    "raw_counts": {
      "total": 4,
      "media": 4
    },
    "uploaded": [
      "mxc://example.org/abc123",
      "mxc://example.org/abc124",
      "mxc://example.org/abc125"
    ]
  }
}
```

**Note**: The endpoint may return values which represent duplicated media across itself and other hosts.

**Note**: Thumbnails are not associated with users and therefore are not included by this endpoint.

#### Per-user usage (batch of users / single user)

Use the same endpoint as above, but specifying one or more `?user_id=@alice:example.org` query parameters. Note that encoding the values may be required (not shown here). Users that are unknown to the media repo will not be returned.

#### Per-upload usage (all uploads)

URL: `GET /_matrix/media/unstable/admin/usage/<server name>/uploads?access_token=your_access_token`

The response is how much data the server is using:
```json
{
  "mxc://example.org/abc123": {
    "size_bytes": 102400,
    "uploaded_by": "@alice:example.org",
    "datastore_id": "def456",
    "datastore_location": "/var/media-repo/ab/cd/12345",
    "sha256_hash": "ghi789",
    "quarantined": false,
    "upload_name": "info.txt",
    "content_type": "text/plain",
    "created_ts": 1561514528225
  }
}
```

#### Per-upload usage (batch of uploads / single upload)

Use the same endpoint as above, but specifying one or more `?mxc=mxc://example.org/abc123` query parameters. Note that encoding the values may be required (not shown here).

Only repository administrators can use these endpoints.

## Background Tasks API

The media repo keeps track of tasks that were started and did not block the request. For example, transferring media or quarantining large amounts of media may result in a background task. A `task_id` will be returned by those endpoints which can then be used here to get the status of a task.

#### Listing all tasks

URL: `GET /_matrix/media/unstable/admin/tasks/all`

The response is a list of all known tasks:
```json
[
  {
    "task_id": 1,
    "task_name": "storage_migration",
    "params": {
      "before_ts": 1567460189817,
      "source_datastore_id": "abc123",
      "target_datastore_id": "def456"
    },
    "start_ts": 1567460189913,
    "end_ts": 1567460190502,
    "is_finished": true
  },
  {
    "task_id": 2,
    "task_name": "storage_migration",
    "params": {
      "before_ts": 1567460189817,
      "source_datastore_id": "ghi789",
      "target_datastore_id": "123abc"
    },
    "start_ts": 1567460189913,
    "end_ts": 0,
    "is_finished": false
  }
]
```

**Note**: The `params` vary depending on the task.

#### Listing unfinished tasks

URL: `GET /_matrix/media/unstable/admin/tasks/unfinished`

The response is a list of all unfinished tasks:
```json
[
  {
    "task_id": 2,
    "task_name": "storage_migration",
    "params": {
      "before_ts": 1567460189817,
      "source_datastore_id": "ghi789",
      "target_datastore_id": "123abc"
    },
    "start_ts": 1567460189913,
    "end_ts": 0,
    "is_finished": false
  }
]
```

**Note**: The `params` vary depending on the task.

#### Getting information on a specific task

URL: `GET /_matrix/media/unstable/admin/tasks/<task ID>`

The response is the status of the task:
```json
{
  "task_id": 1,
  "task_name": "storage_migration",
  "params": {
    "before_ts": 1567460189817,
    "source_datastore_id": "abc123",
    "target_datastore_id": "def456"
  },
  "start_ts": 1567460189913,
  "end_ts": 1567460190502,
  "is_finished": true
}
```

**Note**: The `params` vary depending on the task.
