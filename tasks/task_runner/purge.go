package task_runner

import (
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/util"
)

type purgeConfig struct {
	IncludeQuarantined bool
}

type PurgeAuthContext struct {
	UploaderUserId string
	SourceOrigin   string
}

func (c *PurgeAuthContext) canAffect(media *database.DbMedia) bool {
	if c.UploaderUserId != "" && c.UploaderUserId != media.UserId {
		return false
	}
	if c.SourceOrigin != "" && c.SourceOrigin != media.Origin {
		return false
	}
	return true
}

func PurgeMedia(ctx rcontext.RequestContext, authContext *PurgeAuthContext, toHandles []*QuarantineThis) ([]string, error) {
	records := make([]*database.DbMedia, 0)

	for _, toHandle := range toHandles {
		record, err := resolveMedia(ctx, "", toHandle)
		if err != nil {
			return nil, err
		}

		records = append(records, record...)
	}

	// Check auth on all records before actually processing them
	for _, r := range records {
		if !authContext.canAffect(r) {
			return nil, common.ErrWrongUser
		}
	}

	// Now we process all the records
	return doPurge(ctx.AsBackground(), records, &purgeConfig{IncludeQuarantined: true})
}

func doPurge(ctx rcontext.RequestContext, records []*database.DbMedia, config *purgeConfig) ([]string, error) {
	mediaDb := database.GetInstance().Media.Prepare(ctx)
	thumbsDb := database.GetInstance().Thumbnails.Prepare(ctx)
	attrsDb := database.GetInstance().MediaAttributes.Prepare(ctx)
	reservedDb := database.GetInstance().ReservedMedia.Prepare(ctx)

	// Filter the records early on to remove things we're not going to handle
	ctx.Log.Debug("Purge pre-filter")
	records2 := make([]*database.DbMedia, 0)
	for _, r := range records {
		if r.Quarantined && !config.IncludeQuarantined {
			continue // skip quarantined media so later loops don't try to purge it
		}
		attrs, err := attrsDb.Get(r.Origin, r.MediaId)
		if err != nil {
			return nil, err
		}
		if attrs != nil && attrs.Purpose == database.PurposePinned {
			continue
		}

		records2 = append(records2, r)
	}
	records = records2

	flagMap := make(map[string]map[string]bool) // outer key = file location, inner key = MXC, value = in records[]
	thumbsMap := make(map[string][]*database.DbThumbnail)

	// First, we identify all the media which is using the file references we think we want to delete
	// This includes thumbnails (flagged under the original media MXC URI)
	ctx.Log.Debug("Stage 1 of purge")
	doFlagging := func(datastoreId string, location string) error {
		locationId := fmt.Sprintf("%s/%s", datastoreId, location)
		if _, ok := flagMap[locationId]; ok {
			return nil // we already processed this file location - skip trying to populate from it
		}

		flagMap[locationId] = make(map[string]bool)

		// Find media records first
		media, err := mediaDb.GetByLocation(datastoreId, location)
		if err != nil {
			return err
		}
		for _, r2 := range media {
			mxc := util.MxcUri(r2.Origin, r2.MediaId)
			flagMap[locationId][mxc] = false
		}

		// Now thumbnails
		thumbs, err := thumbsDb.GetByLocation(datastoreId, location)
		if err != nil {
			return err
		}
		for _, r2 := range thumbs {
			mxc := util.MxcUri(r2.Origin, r2.MediaId)
			flagMap[locationId][mxc] = false
		}

		return nil
	}
	for _, r := range records {
		if err := doFlagging(r.DatastoreId, r.Location); err != nil {
			return nil, err
		}

		// We also grab all the thumbnails of the proposed media to clear those files out safely too
		thumbs, err := thumbsDb.GetForMedia(r.Origin, r.MediaId)
		if err != nil {
			return nil, err
		}
		thumbsMap[util.MxcUri(r.Origin, r.MediaId)] = thumbs
		for _, t := range thumbs {
			if err = doFlagging(t.DatastoreId, t.Location); err != nil {
				return nil, err
			}
		}
	}

	// Next, we re-iterate to flag records as being deleted
	ctx.Log.Debug("Stage 2 of purge")
	markBeingPurged := func(locationId string, mxc string) error {
		if m, ok := flagMap[locationId]; !ok {
			return errors.New("logic error: missing flag map for location ID in second step")
		} else {
			if v, ok := m[mxc]; !ok {
				return errors.New("logic error: missing flag map value for MXC URI in second step")
			} else if !v { // if v is `true` then it's already been processed - skip a write step
				m[mxc] = true
			}
		}

		return nil
	}
	for _, r := range records {
		locationId := fmt.Sprintf("%s/%s", r.DatastoreId, r.Location)
		mxc := util.MxcUri(r.Origin, r.MediaId)
		if err := markBeingPurged(locationId, mxc); err != nil {
			return nil, err
		}

		// Mark the thumbnails too
		if thumbs, ok := thumbsMap[mxc]; !ok {
			return nil, errors.New("logic error: missing thumbnails map value for MXC URI in second step")
		} else {
			for _, t := range thumbs {
				locationId = fmt.Sprintf("%s/%s", t.DatastoreId, t.Location)
				mxc = util.MxcUri(t.Origin, t.MediaId)
				if err := markBeingPurged(locationId, mxc); err != nil {
					return nil, err
				}
			}
		}
	}

	// Finally, we can run through the records and start deleting media that's safe to delete
	ctx.Log.Debug("Stage 3 of purge")
	deletedLocations := make(map[string]bool)
	removedMxcs := make([]string, 0)
	tryRemoveDsFile := func(datastoreId string, location string) error {
		locationId := fmt.Sprintf("%s/%s", datastoreId, location)
		if _, ok := deletedLocations[locationId]; ok {
			return nil // already deleted/handled
		}
		if m, ok := flagMap[locationId]; !ok {
			return errors.New("logic error: missing flag map value for location ID in third step")
		} else {
			for _, b := range m {
				if !b {
					return nil // unsafe to delete, but no error
				}
			}
		}

		// Try deleting the file
		err := datastores.RemoveWithDsId(ctx, datastoreId, location)
		if err != nil {
			return err
		}
		deletedLocations[locationId] = true
		return nil
	}
	for _, r := range records {
		mxc := util.MxcUri(r.Origin, r.MediaId)

		if err := tryRemoveDsFile(r.DatastoreId, r.Location); err != nil {
			return nil, err
		}
		if util.IsServerOurs(r.Origin) {
			if err := reservedDb.InsertNoConflict(r.Origin, r.MediaId, "purged / deleted"); err != nil {
				return nil, err
			}
		}
		if !r.Quarantined { // keep quarantined flag
			if err := mediaDb.Delete(r.Origin, r.MediaId); err != nil {
				return nil, err
			}
		}
		removedMxcs = append(removedMxcs, mxc)

		// Remove the thumbnails too
		if thumbs, ok := thumbsMap[mxc]; !ok {
			return nil, errors.New("logic error: missing thumbnails for MXC URI in third step")
		} else {
			for _, t := range thumbs {
				if err := tryRemoveDsFile(t.DatastoreId, t.Location); err != nil {
					return nil, err
				}
				if err := thumbsDb.Delete(t); err != nil {
					return nil, err
				}
			}
		}
	}

	// Finally, we're done
	return removedMxcs, nil
}
