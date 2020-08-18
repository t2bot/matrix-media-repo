package unstable

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy"
)

func IPFSDownload(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	ipfsContentId := params["ipfsContentId"]

	targetDisposition := r.URL.Query().Get("org.matrix.msc2702.asAttachment")
	if targetDisposition == "true" {
		targetDisposition = "attachment"
	} else if targetDisposition == "false" {
		targetDisposition = "inline"
	} else {
		targetDisposition = "infer"
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"ipfsContentId": ipfsContentId,
		"server":        server,
	})

	obj, err := ipfs_proxy.GetObject(ipfsContentId, rctx)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("unexpected error")
	}

	return &r0.DownloadMediaResponse{
		ContentType:       obj.ContentType,
		Filename:          obj.FileName,
		SizeBytes:         obj.SizeBytes,
		Data:              obj.Data,
		TargetDisposition: targetDisposition,
	}
}
