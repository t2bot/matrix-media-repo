# Media repository administration

All the API calls here require your user ID to be listed in the configuration as an administrator. After that, your access token for your homeserver will grant you access to these APIs. The URLs should be hit against a configured homeserver. For example, if you have `t2bot.io` configured as a homeserver, then the admin API can be used at `https://t2bot.io/_matrix/media/r0/admin/...`.

## Remote media purge

URL: `POST /_matrix/media/r0/admin/purge_remote?before_ts=1234567890&access_token=your_access_token` (`before_ts` is in milliseconds)

This will delete remote media from the file store that was downloaded before the timestamp specified. If the file is referenced by newer remote media or local files to any of the configured homeservers, it will not be deleted. Be aware that removing a homeserver from the config will cause it to be considered a remote server, and therefore the media may be deleted.

Any remote media that is deleted and requested by a user will be downloaded again.

## Quarantine media

URL: `POST /_matrix/media/r0/admin/quarantine/<server>/<media id>?access_token=your_access_token`

The `<server>` and `<media id>` can be retrieved from an MXC URI (`mxc://<server>/<media id>`).

The quarantine media API allows administrators to quarantine media that may not be appropriate for their server. Using this API will prevent the media from being downloaded any further. It will *not* delete the file from your storage though: that is a task left for the administrator.

Remote media that has been quarantined will not be purged either. This is so that the media remains flagged as quarantined. It is safe to delete the file on your disk, but not delete the media from the database.

Quarantining media will also quarantine any media with the same file hash.
