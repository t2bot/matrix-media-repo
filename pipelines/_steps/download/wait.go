package download

import (
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/notifier"
)

func WaitForAsyncMedia(ctx rcontext.RequestContext, origin string, mediaId string) (*database.DbMedia, error) {
	db := database.GetInstance().ExpiringMedia.Prepare(ctx)
	record, err := db.Get(origin, mediaId)
	if err != nil {
		return nil, err
	}
	if record == nil || record.IsExpired() {
		return nil, nil // there's not going to be a record
	}

	ch, finish := notifier.GetUploadWaitChannel(origin, mediaId)
	defer finish()
	select {
	case <-ctx.Context.Done():
		return nil, common.ErrMediaNotYetUploaded
	case val := <-ch:
		return val, nil
	}
}
