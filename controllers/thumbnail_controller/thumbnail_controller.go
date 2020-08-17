package thumbnail_controller

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/disintegration/imaging"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/globals"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/controllers/quarantine_controller"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/thumbnailing"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

var localCache = cache.New(30*time.Second, 60*time.Second)

func GetThumbnail(origin string, mediaId string, desiredWidth int, desiredHeight int, animated bool, method string, downloadRemote bool, ctx rcontext.RequestContext) (*types.StreamedThumbnail, error) {
	media, err := download_controller.FindMediaRecord(origin, mediaId, downloadRemote, ctx)
	if err != nil {
		return nil, err
	}

	mediaContentType := util.FixContentType(media.ContentType)

	if !thumbnailing.IsSupported(mediaContentType) {
		ctx.Log.Warn("Cannot generate thumbnail for " + mediaContentType + " because it is not supported")
		return nil, errors.New("cannot generate thumbnail for this media's content type")
	}

	if !util.ArrayContains(ctx.Config.Thumbnails.Types, mediaContentType) {
		ctx.Log.Warn("Cannot generate thumbnail for " + mediaContentType + " because it is not listed in the config")
		return nil, errors.New("cannot generate thumbnail for this media's content type")
	}

	if media.Quarantined {
		ctx.Log.Warn("Quarantined media accessed")

		if ctx.Config.Quarantine.ReplaceThumbnails {
			ctx.Log.Info("Replacing thumbnail with a quarantined one")

			img, err := quarantine_controller.GenerateQuarantineThumbnail(desiredWidth, desiredHeight, ctx)
			if err != nil {
				return nil, err
			}

			data := &bytes.Buffer{}
			_ = imaging.Encode(data, img, imaging.PNG)
			return &types.StreamedThumbnail{
				Stream: util.BufferToStream(data),
				Thumbnail: &types.Thumbnail{
					// We lie about the details to ensure we keep our contract
					Width:       img.Bounds().Max.X,
					Height:      img.Bounds().Max.Y,
					MediaId:     media.MediaId,
					Origin:      media.Origin,
					Location:    "",
					ContentType: "image/png",
					Animated:    false,
					Method:      method,
					CreationTs:  util.NowMillis(),
					SizeBytes:   int64(data.Len()),
				},
			}, nil
		}

		return nil, common.ErrMediaQuarantined
	}

	if animated && ctx.Config.Thumbnails.MaxAnimateSizeBytes > 0 && ctx.Config.Thumbnails.MaxAnimateSizeBytes < media.SizeBytes {
		ctx.Log.Warn("Attempted to animate a media record that is too large. Assuming animated=false")
		animated = false
	}

	if animated && !thumbnailing.IsAnimationSupported(mediaContentType) {
		ctx.Log.Warn("Attempted to animate a media record that isn't an animated type. Assuming animated=false")
		animated = false
	}

	if media.SizeBytes > ctx.Config.Thumbnails.MaxSourceBytes {
		ctx.Log.Warn("Media too large to thumbnail")
		return nil, common.ErrMediaTooLarge
	}

	width, height, method, err := pickThumbnailDimensions(desiredWidth, desiredHeight, method, ctx)
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%s/%s?w=%d&h=%d&m=%s&a=%t", media.Origin, media.MediaId, width, height, method, animated)

	v, _, err := globals.DefaultRequestGroup.Do(cacheKey, func() (interface{}, error) {
		db := storage.GetDatabase().GetThumbnailStore(ctx)

		var thumbnail *types.Thumbnail
		item, found := localCache.Get(cacheKey)
		if found {
			thumbnail = item.(*types.Thumbnail)
		} else {
			ctx.Log.Info("Getting thumbnail record from database")
			dbThumb, err := db.Get(media.Origin, media.MediaId, width, height, method, animated)
			if err != nil {
				if err == sql.ErrNoRows {
					ctx.Log.Info("Thumbnail does not exist, attempting to generate it")
					genThumb, err2 := GetOrGenerateThumbnail(media, width, height, animated, method, ctx)
					if err2 != nil {
						return nil, err2
					}

					thumbnail = genThumb
				} else {
					return nil, err
				}
			} else {
				thumbnail = dbThumb
			}
		}

		if thumbnail == nil {
			ctx.Log.Warn("Despite all efforts, a thumbnail record could not be found or generated")
			return nil, common.ErrMediaNotFound
		}

		err = storage.GetDatabase().GetMetadataStore(ctx).UpsertLastAccess(thumbnail.Sha256Hash, util.NowMillis())
		if err != nil {
			ctx.Log.Warn("Failed to upsert the last access time: ", err)
		}

		localCache.Set(cacheKey, thumbnail, cache.DefaultExpiration)

		cached, err := internal_cache.Get().GetMedia(thumbnail.Sha256Hash, internal_cache.StreamerForThumbnail(thumbnail), ctx)
		if err != nil {
			return nil, err
		}
		if cached != nil && cached.Contents != nil {
			return &types.StreamedThumbnail{
				Thumbnail: thumbnail,
				Stream:    ioutil.NopCloser(cached.Contents),
			}, nil
		}

		ctx.Log.Info("Reading thumbnail from datastore")
		mediaStream, err := datastore.DownloadStream(ctx, thumbnail.DatastoreId, thumbnail.Location)
		if err != nil {
			return nil, err
		}

		return &types.StreamedThumbnail{Thumbnail: thumbnail, Stream: mediaStream}, nil
	}, func(v interface{}, count int, err error) []interface{} {
		if err != nil {
			return nil
		}

		rv := v.(*types.StreamedThumbnail)
		vals := make([]interface{}, 0)
		streams := util.CloneReader(rv.Stream, count)

		for i := 0; i < count; i++ {
			internal_cache.Get().MarkDownload(rv.Thumbnail.Sha256Hash)
			vals = append(vals, &types.StreamedThumbnail{
				Thumbnail: rv.Thumbnail,
				Stream:    streams[i],
			})
		}

		return vals
	})

	var value *types.StreamedThumbnail
	if v != nil {
		value = v.(*types.StreamedThumbnail)
	}

	return value, err
}

func GetOrGenerateThumbnail(media *types.Media, width int, height int, animated bool, method string, ctx rcontext.RequestContext) (*types.Thumbnail, error) {
	db := storage.GetDatabase().GetThumbnailStore(ctx)
	thumbnail, err := db.Get(media.Origin, media.MediaId, width, height, method, animated)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err != sql.ErrNoRows {
		ctx.Log.Info("Using thumbnail from database")
		return thumbnail, nil
	}

	ctx.Log.Info("Generating thumbnail")

	thumbnailChan := getResourceHandler().GenerateThumbnail(media, width, height, method, animated)
	defer close(thumbnailChan)

	result := <-thumbnailChan
	return result.thumbnail, result.err
}

func pickThumbnailDimensions(desiredWidth int, desiredHeight int, desiredMethod string, ctx rcontext.RequestContext) (int, int, string, error) {
	if desiredWidth <= 0 {
		return 0, 0, "", errors.New("width must be positive")
	}
	if desiredHeight <= 0 {
		return 0, 0, "", errors.New("height must be positive")
	}
	if desiredMethod != "crop" && desiredMethod != "scale" {
		return 0, 0, "", errors.New("method must be crop or scale")
	}

	foundSize := false
	targetWidth := 0
	targetHeight := 0
	largestWidth := 0
	largestHeight := 0
	desiredAspectRatio := float32(desiredWidth) / float32(desiredHeight)

	for _, size := range ctx.Config.Thumbnails.Sizes {
		largestWidth = util.MaxInt(largestWidth, size.Width)
		largestHeight = util.MaxInt(largestHeight, size.Height)

		// Unlikely, but if we get the exact dimensions then just use that
		if desiredWidth == size.Width && desiredHeight == size.Height {
			return size.Width, size.Height, desiredMethod, nil
		}

		// If we come across a size that's larger than requested, try and use that
		if desiredWidth <= size.Width && desiredHeight <= size.Height {
			// Only use our new found size if it's smaller than one we've already picked
			if !foundSize || (targetWidth > size.Width && targetHeight > size.Height) {
				targetWidth = size.Width
				targetHeight = size.Height
				foundSize = true
			}
		}
	}

	if ctx.Config.Thumbnails.DynamicSizing {
		return util.MinInt(largestWidth, desiredWidth), util.MinInt(largestHeight, desiredHeight), desiredMethod, nil
	}

	// Use the largest dimensions available if we didn't find anything
	if !foundSize {
		targetWidth = largestWidth
		targetHeight = largestHeight
	}

	if desiredMethod == "crop" {
		// We need to maintain the aspect ratio of the request
		sizeAspect := float32(targetWidth) / float32(targetHeight)
		if sizeAspect != desiredAspectRatio { // it's unlikely to match, but we can dream
			ratio := util.MinFloat32(float32(targetWidth)/float32(desiredWidth), float32(targetHeight)/float32(desiredHeight))
			targetWidth = int(float32(desiredWidth) * ratio)
			targetHeight = int(float32(desiredHeight) * ratio)
		}
	}

	return targetWidth, targetHeight, desiredMethod, nil
}
