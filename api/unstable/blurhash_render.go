package unstable

import (
	"bytes"
	"image/png"
	"net/http"
	"strconv"

	"github.com/buckket/go-blurhash"
	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

func RenderBlurhash(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	hash := params["blurhash"]

	width := rctx.Config.Features.MSC2448Blurhash.MaxRenderWidth / 2
	height := rctx.Config.Features.MSC2448Blurhash.MaxRenderHeight / 2

	wstr := r.URL.Query().Get("width")
	hstr := r.URL.Query().Get("height")

	if wstr != "" {
		i, err := strconv.Atoi(wstr)
		if err != nil {
			return api.BadRequest("width must be an integer")
		}
		width = i
	}
	if hstr != "" {
		i, err := strconv.Atoi(hstr)
		if err != nil {
			return api.BadRequest("height must be an integer")
		}
		height = i
	}

	if width > rctx.Config.Features.MSC2448Blurhash.MaxRenderWidth {
		width = rctx.Config.Features.MSC2448Blurhash.MaxRenderWidth
	}
	if height > rctx.Config.Features.MSC2448Blurhash.MaxRenderHeight {
		height = rctx.Config.Features.MSC2448Blurhash.MaxRenderHeight
	}

	img, err := blurhash.Decode(hash, width, height, rctx.Config.Features.MSC2448Blurhash.Punch)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("Unexpected error rendering blurhash")
	}
	buf := &bytes.Buffer{}
	err = png.Encode(buf, img)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("Unexpected error rendering blurhash")
	}

	return &r0.DownloadMediaResponse{
		ContentType: "image/png",
		Filename:    "blurhash.png",
		SizeBytes:   int64(buf.Len()),
		Data:        util.BufferToStream(buf), // convert to stream to avoid console spam
		TargetDisposition: "inline",
	}
}
