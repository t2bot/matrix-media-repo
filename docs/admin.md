# Media repository administration

All the API calls here require your user ID to be listed in the configuration as an administrator. After that, your access token for your homeserver will grant you access to these APIs. The URLs should be hit against a configured homeserver. For example, if you have `t2bot.io` configured as a homeserver, then the admin API can be used at `https://t2bot.io/_matrix/media/r0/admin/...`.

## Remote media purge

URL: `POST /_matrix/media/unstable/admin/purge_remote?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete remote media from the file store that was downloaded before the timestamp specified. If the file is referenced by newer remote media or local files to any of the configured homeservers, it will not be deleted. Be aware that removing a homeserver from the config will cause it to be considered a remote server, and therefore the media may be deleted.

Any remote media that is deleted and requested by a user will be downloaded again.

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
