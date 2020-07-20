# Redis support

**Note**: Redis support is currently experimental and not intended for usage in general cases.

Redis can be used as a high-performance cache for the media repo, allowing for (in future) multiple media
repositories to run concurrently in limited jobs (such as some processes handling uploads, others downloads,
etc). Currently though, it is capable of speeding up deployments of disjointed media repos or in preparation
for proper load balancer support by the media repo.

The media repo connects to a number of "shards" (Redis processes) to distribute cached keys over them. Each
shard is not expected to store persistent data and should be tolerant of total failure - the media repo assumes
that the shards will be dedicated to caching and thus will not have any expectations that a particular shard
will remain running.

Setting up a shard is fairly simple: it's the same as deploying Redis itself. The media repo does not manage
the expiration policy for the shards, so it is recommended to give https://redis.io/topics/lru-cache a read to
pick the best eviction policy for your environment. The current recommendations are:
* A `maxmemory` of at least `1gb` for each shard.
* A `maxmemory-policy` of `allkeys-lfu` to ensure that the cache gets cleared out (the media repo does not set
  an expiration time or TTL). Note: an `lru` mode is *not* recommended as the media repo will be caching all
  uploads it sees, which includes remote media. A `lfu` mode ensures that recent items being cached can still
  be evicted if not commonly requested.
* 1 shard for most deployments. Larger repos (or connecting many media repos to the same shards) should consider
  3 or more shards.

The shards in the ring can be changed at runtime by updating the config and ensuring the media repo has reloaded
the config. Note that changing cache mechanisms at runtime is not recommended, and a full restart is recommended
instead.

**Note**: Metrics reported for cache size will be inaccurate. Frequencies of requests will still be reported.

**Note**: Quarantined media will still be stored in the cache. This is considered a bug and will need fixing.

## Connecting multiple disjointed media repos

Though the media repo expects to be the sole and only thing in the datacenter handling media, it is not always
possible or sane to do so. Examples including hosting providers which may have several media repos handling a
small subset of domains each. In these scenarios, it may be beneficial to set up a series of Redis shards within
each datacenter and connect all the media repos in that DC to them. This can reduce the amount of time it takes
to retrieve media from a media repo in that DC, as well as avoid downloading several copies of remote media.

Note that even when connecting media repos to the same set of shards the repos will still attempt to upload a
copy of the media to the datastore. For example, if media repo A downloads something from matrix.org and puts
it into the cache, media repo B will first get it from the cache and upload it to its datastore when a user 
requests the same media. The benefit, however, is that only 1 request to matrix.org happened instead of two.

All media repos should be connected to the same set of shards to ensure even balancing between the shards.
Additionally, all media repos **must** be running the same major version (anything in `1.x.x`) in order to avoid 
conflicts.
