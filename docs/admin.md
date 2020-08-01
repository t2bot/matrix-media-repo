# Media repository administration

All the API calls here require your user ID to be listed in the configuration as an administrator. After that, your access token for your homeserver will grant you access to these APIs. The URLs should be hit against a configured homeserver. For example, if you have `t2bot.io` configured as a homeserver, then the admin API can be used at `https://t2bot.io/_matrix/media/unstable/admin/...`.

## Media attributes

Media in the media repo can have attributes associated with it.

Currently the only attribute is a purpose, which defines how the media repo is to treat the media. By default this is set
to `none`, meaning the media repo will not treat it as special in any way. Setting the purpose to `pinned` will prevent
the media from being quarantined, but not purged.

#### Get media attributes

URL: `GET /_matrix/media/unstable/admin/media/<server>/<media id>/attributes?access_token=your_access_token`

The response will be the current attributes for the media.

#### Set media attributes

URL: `POST /_matrix/media/unstable/admin/media/<server>/<media id>/attributes/set?access_token=your_access_token`

The request body will be the new attributes for the media. It is recommended to first get the attributes before setting them.

## Media purge

Sometimes you just want your disk space back - purging media is the best way to do that. **Be careful about what you're purging.** The media repo will happily purge a local media object, making it highly unlikely to ever exist in Matrix again. When the media repo deletes remote media, it is only deleting its copy of it - it cannot delete media on the remote server itself. Thumbnails will also be deleted for the media.

If the file is duplicated over many media records, it will not be physically deleted (however the media record that was purged will be counted as deleted). The exception to this is quarantined media: when the record being purged is also quarantined, the media is deleted from the datastore even if it is duplicated in multiple records.

#### Purge remote media

URL: `POST /_matrix/media/unstable/admin/purge/remote?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete remote media from the file store that was downloaded before the timestamp specified. If the file is referenced by newer remote media or local files to any of the configured homeservers, it will not be deleted. Be aware that removing a homeserver from the config will cause it to be considered a remote server, and therefore the media may be deleted.

Any remote media that is deleted and requested by a user will be downloaded again.

This endpoint is only available to repository administrators.

#### Purge quarantined media

URL: `POST /_matrix/media/unstable/admin/purge/quarantined?access_token=your_access_token`

This will delete all media that has previously been quarantined, local or remote. If called by a homeserver administrator (who is not a repository administrator), only content quarantined for their domain will be purged.

#### Purge individual record

URL: `POST /_matrix/media/unstable/admin/purge/<server>/<media id>?access_token=your_access_token`

This will delete the media record, regardless of it being local or remote. Can be called by homeserver administrators and the uploader to delete it.

#### Purge media uploaded by user

URL: `POST /_matrix/media/unstable/admin/purge/user/<user id>?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete all media uploaded by that user before the timestamp specified. Can be called by homeserver administrators, if they own the user ID being purged.

#### Purge media uploaded in a room

URL: `POST /_matrix/media/unstable/admin/purge/room/<room id>?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete all media known to that room, regardless of it being local or remote, before the timestamp specified. If called by a homeserver administrator, only media uploaded to their domain will be deleted.

#### Purge media uploaded by a server

URL: `POST /_matrix/media/unstable/admin/purge/server/<server name>?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete all media known to be uploaded by that server, regardless of it being local or remote, before the timestamp specified. If called by a homeserver administrator, only media uploaded to their domain will be deleted.

#### Purge media that hasn't been accessed in a while

URL: `POST /_matrix/media/unstable/admin/purge/old?before_ts=1234567890&include_local=false&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete all media that hasn't been accessed since `before_ts` (defaults to 'now'). If `include_local` is `false` (the default), only remote media will be deleted.

This endpoint is only available to repository administrators.

## Quarantine media

The quarantine media API allows administrators to quarantine media that may not be appropriate for their server. Using this API will prevent the media from being downloaded any further. It will *not* delete the file from your storage though: that is a task left for the administrator.

Remote media that has been quarantined will not be purged either. This is so that the media remains flagged as quarantined. It is safe to delete the file on your disk, but not delete the media from the database.

Quarantining media will also quarantine any media with the same file hash.

This API is unique in that it can allow administrators of configured homeservers to quarantine media on their homeserver only. This will not allow local administrators to quarantine remote media or media on other homeservers though, just on theirs.

#### Quarantine a specific record

URL: `POST /_matrix/media/unstable/admin/quarantine/<server>/<media id>?access_token=your_access_token`

The `<server>` and `<media id>` can be retrieved from an MXC URI (`mxc://<server>/<media id>`).

#### Quarantine a whole room's worth of media

URL: `POST /_matrix/media/unstable/admin/quarantine/room/<room id>?access_token=your_access_token`

#### Quarantine a whole user's worth of media

URL: `POST /_matrix/media/unstable/admin/quarantine/user/<user id>?access_token=your_access_token`

#### Quarantine a whole server's worth of media

URL: `POST /_matrix/media/unstable/admin/quarantine/server/<server name>?access_token=your_access_token`

Note that this will only quarantine what is currently known to the repo. It will not flag the domain for future quarantines.

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

## Exporting/Importing data

Exports (and therefore imports) are currently done on a per-user basis. This is primarily useful when moving users to new hosts or doing GDPR exports of user data.

#### Exporting data for a user

URL: `POST /_matrix/media/unstable/admin/user/<user ID>/export?include_data=true&s3_urls=true`

Both query params are optional, and their default values are shown. If `include_data` is false, only metadata will be returned by the export. `s3_urls`, when true, includes the s3 URL to the media in the metadata if one is available.

The response is a task ID and export ID to put into the 'view export' URL:

```json
{
  "export_id": "abcdef",
  "task_id": 12
}
```

**Note**: the `export_id` will be included in the task's `params`.

**Note**: the `export_id` should be treated as a secret/authentication token as it allows someone to download other people's data.

#### Exporting data for a domain

URL: `POST /_matrix/media/unstable/admin/server/<server name>/export?include_data=true&s3_urls=true`

Response is the same as the user export endpoint above. The `<server name>` does not need to be configured in the repo - it will export data it has on a remote server if you ask it to.

#### Viewing an export

After the task has been completed, the `export_id` can be used to download the content.

URL: `GET /_matrix/media/unstable/admin/export/<export ID>/view`

The response will be a webpage for the user to interact with. From this page, the user can say they've downloaded the export and delete it.

#### Downloading an export (for scripts)

Similar to viewing an export, an export may be downloaded to later be imported.

Exports are split into several tar (gzipped) files and need to be downloaded individually. To get the list of files, call:

`GET /_matrix/media/unstable/admin/export/<export ID>/metadata`

which returns:

```json
{
  "entity": "@travis:t2l.io",
  "parts": [
    {
      "index": 1,
      "size": 1024000,
      "name": "TravisR-part-1.tgz"
    },
    {
      "index": 2,
      "size": 1024000,
      "name": "TravisR-part-2.tgz"
    }
  ]
}
```

**Note**: the `name` demonstrated may be different and should not be parsed. The `size` is in bytes.

Then one can call the following to download each part:

`GET /_matrix/media/unstable/admin/export/<export ID>/part/<index>`

#### Deleting an export

After the export has been downloaded, it can be deleted. Note that this endpoint can be called by the user from the "view export" page.

`DELETE /_matrix/media/unstable/admin/export/<export ID>`

The response is an empty JSON object if successful.

#### Importing a previous export

Once an export has been completed it can be imported back into the media repo. Files that are already known to the repo will not be overwritten - it'll use its known copy first.

**Note**: Imports happen in memory, which can balloon quickly depending on how you exported your data. Although you can import data without s3 it is recommended that you only import from archives generated with `include_data=false`.

**Note**: Only repository administrators can perform imports, regardless of who they are for.

**Note**: Imports done through this method can affect other homeservers! For example, a user's data export could contain
an entry for a homeserver other than their own, which the media repo will happily import. Always validate the manifest
of an import before running it!

URL: `POST /_matrix/media/unstable/admin/import`

The request body is the bytes of the first archive (eg: `TravisR-part-1.tgz` in the above examples).

The response body will be something like the following: 
```json
{ 
  "import_id": "abcdef",
  "task_id": 13
}
```

**Note**: the `import_id` will be included in the task's `params`.

**Note**: the `import_id` should be treated as a secret/authentication token as it could allow for an attacker to change what the user has uploaded.

To import the subsequent parts of an export, use the following endpoint and supply the archive as the request body: `POST /_matrix/media/unstable/admin/import/<import ID>/part`

The parts can be uploaded in any order and will be extracted in memory.

Imports will look for the files included from the archives, though if an S3 URL is available and the file isn't found it will use that instead. If the S3 URL points at a known datastore for the repo, it will assume the file exists and use that location without pulling it into memory.

Imports stay open until all files have been imported (or until the process crashes). This also means you can upload the parts at your leisure instead of trying to push all the data up to the server as fast as possible. If the task is still considered running, the import is still open.

**Note**: When using s3 URLs to do imports it is possible for the media to bypass checks like allowed file types, maximum sizes, and quarantines.

#### Closing an import manually

If you have no intention of continuing an import, use this endpoint.

URL: `POST /_matrix/media/unstable/admin/import/<import ID>/close`

The import will be closed and stop waiting for new files to show up. It will continue importing whatever files it already knows about - to forcefully end this task simply restart the process.
