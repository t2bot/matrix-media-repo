package download_tracker

import (
	"container/list"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/turt2live/matrix-media-repo/util"
)

/*
 * The download tracker works by keeping a limited number of buckets for each media record
 * in an expiring cache. The number of buckets is the equal to the number of minutes to track
 * downloads for (max age). The entire download history expires after the max age so long
 * as it is not added to.
 *
 * The underlying data structure is a linked list of buckets (which are just a timestamp and
 * number of downloads). A linked list was chosen for it's capability to easily append and
 * remove items without having to do array slicing. Buckets were chosen to limit the number
 * of entries in the list because the total downloads are calculated through iteration. If
 * each calculation had to iterate over thousands of items, the execution would be slow. A
 * smaller number, like 30, is a lot more manageable.
 */

type DownloadTracker struct {
	cache  *cache.Cache
	maxAge int64
}

type mediaRecord struct {
	buckets   *list.List
	downloads int
}

type bucket struct {
	ts        int64
	downloads int
}

func New(maxAgeMinutes int) (*DownloadTracker) {
	maxAge := time.Duration(maxAgeMinutes) * time.Minute
	return &DownloadTracker{
		cache:  cache.New(maxAge, maxAge*2),
		maxAge: int64(maxAgeMinutes),
	}
}

func (d *DownloadTracker) NumDownloads(recordId string) (int) {
	item, found := d.cache.Get(recordId)
	if !found {
		return 0
	}

	return d.recountDownloads(item.(*mediaRecord), recordId)
}

func (d *DownloadTracker) Increment(recordId string) (int) {
	item, found := d.cache.Get(recordId)
	var record *mediaRecord
	if !found {
		record = &mediaRecord{buckets: list.New()}
	} else {
		record = item.(*mediaRecord)
	}

	bucketTs := util.NowMillis() / 60000 // minutes

	if record.buckets.Len() <= 0 {
		// First bucket
		record.buckets.PushFront(&bucket{
			ts:        bucketTs,
			downloads: 1,
		})
	} else {
		firstRecord := record.buckets.Front().Value.(*bucket)
		if firstRecord.ts != bucketTs {
			record.buckets.PushFront(&bucket{
				ts:        bucketTs,
				downloads: 1,
			})
		} else {
			firstRecord.downloads++
		}
	}

	return d.recountDownloads(record, recordId)
}

func (d *DownloadTracker) recountDownloads(record *mediaRecord, recordId string) int {
	currentBucketTs := util.NowMillis() / 60000 // minutes
	changed := false

	// Trim off anything that is too old
	for e := record.buckets.Back(); e != nil; e = record.buckets.Back() {
		b := e.Value.(*bucket)
		if (currentBucketTs - b.ts) > d.maxAge {
			changed = true
			record.buckets.Remove(e)
		} else {
			break // count is still relevant
		}
	}

	// Count the number of downloads
	downloads := 0
	for e := record.buckets.Front(); e != nil; e = e.Next() {
		b := e.Value.(*bucket)
		downloads += b.downloads
	}

	if changed || downloads != record.downloads {
		record.downloads = downloads
		d.cache.Set(recordId, record, cache.DefaultExpiration)
	}

	return downloads
}
