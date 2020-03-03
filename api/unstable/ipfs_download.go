package unstable

import (
	"bytes"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

func IPFSDownload(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	ipfsContentId := params["ipfsContentId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"ipfsContentId": ipfsContentId,
		"server":        server,
	})

	ipfs, err := httpapi.NewLocalApi()
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("Unexpected error connecting to IPFS")
	}

	ipfsCid, err := cid.Decode(ipfsContentId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("Unexpected error decoding content ID")
	}

	ipfsPath := path.IpfsPath(ipfsCid)
	node, err := ipfs.ResolveNode(rctx.Context, ipfsPath)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("Unexpected error resolving object from IPFS")
	}

	return &r0.DownloadMediaResponse{
		ContentType: "application/octet-stream",
		Filename:    "ipfs.dat", // TODO: Figure out how to get a name out of this
		SizeBytes:   int64(len(node.RawData())),
		Data:        util.BufferToStream(bytes.NewBuffer(node.RawData())), // stream to avoid log spam
	}
}
