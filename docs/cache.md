# Cache Strategy

The cache in matrix-media-repo is intended to reduce access times for popular media while also handling spikes in traffic, concurrent downloads, and not running into a memory limit. The cache is completely configurable and may be entirely disabled for smaller deployments (or systems with optimized file systems).

## Approach

The cache works by prioritizing popular media while also preferring large files. This is done by tracking the number of downloads in a given period of time, where higher values are kept in cache longer. The cache will consider storing a file in memory once a minimum number of downloads is reached to prevent eager memory usage. 

Media is only permitted in the cache if it also meets a maximum size requirement. The cache is limited to only consume a given amount of memory and therefore extremely large media that monopolize the available space are not welcome. The cache aims to hold the most popular media, and that job is made harder when it can only use 50% of it's available space. In an ideal world, the maximum media size would be greater than or equal to the maximum file size allowed on the server.

An eviction system is also used to help keep larger media in memory. If the cache is full, the smallest and least popular media may be evicted to make room for the new media. This is only done if the cache can prove that evicting the smaller media will make enough room for the new media, otherwise the media is unfortunately not cached. Timers are also added to prevent the cache from bumping media in and out of the cache constantly - media, once evicted, must remain outside the cache for a limited time and new media may not be evicted for a different amount of time (usually 5 times the exclusion time).

## Room composition

The cache performs in the way defined above because most rooms have one of the following approximate compositions:
* A hundred or fewer users belonging to a few servers (communities)
* Hundreds of users belonging to many servers (large communities)
* A hundred or fewer users belonging to many servers (announcement rooms)
* A handful of users belonging to any number of servers (private chats, group of friends) - note: the cache may not kick in for these rooms due to insufficient download counts. File access times should still be suitably low for participants to not notice, however.

The cache considers these cases by optimizing for the spike and followup traffic. In smaller rooms, people are more likely to be downloading media shortly after it is posted while in larger rooms there is an expected spike with a slow decline in download counts over time (as people are less likely to be monitoring the room). In any case, media should be served as quickly as possible - the cache just aims to handle the common room compositions.

Less common, but still viable, compositions are:
* Hundreds of users belonging to a few servers (not federated or large communities) - This is generally seen when the "entry point" to matrix is funneled through a limited number of servers, where stragglers from other servers are not expected. This is an extension of the small room composition.
* Thousands of users belonging to hundreds of servers (Matrix HQ) - This is a relatively rare composition and is best described as an extension of the "large room" problem above. Media in these rooms is likely to be cached for an extremely long time.
