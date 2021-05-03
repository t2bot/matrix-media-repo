package ds_ipfs

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func UploadFile(file io.ReadCloser, ctx rcontext.RequestContext) (*types.ObjectInfo, error) {
	defer cleanup.DumpAndCloseStream(file)

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var cid string
	var writeErr error

	ctx.Log.Info("Writing file...")
	cid, writeErr = ipfs_proxy.PutObject(bytes.NewBuffer(b), ctx)
	if writeErr != nil {
		return nil, writeErr
	}
	ctx.Log.Info("Wrote file to IPFS")

	return &types.ObjectInfo{
		Location: "ipfs/" + cid,
	}, nil
}

func DownloadFile(location string) (io.ReadCloser, error) {
	cid := location[len("ipfs/"):]
	ctx := rcontext.Initial()

	obj, err := ipfs_proxy.GetObject(cid, ctx)
	if err != nil {
		return nil, err
	}

	return obj.Data, nil
}
