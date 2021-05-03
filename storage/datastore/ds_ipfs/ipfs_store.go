package ds_ipfs

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func UploadFile(file io.ReadCloser, ctx rcontext.RequestContext) (*types.ObjectInfo, error) {
	defer cleanup.DumpAndCloseStream(file)

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	done := make(chan bool)
	defer close(done)

	var cid string
	var hash string
	var hashErr error
	var writeErr error

	go func() {
		ctx.Log.Info("Writing file...")
		cid, writeErr = ipfs_proxy.PutObject(bytes.NewBuffer(b), ctx)
		ctx.Log.Info("Wrote file to IPFS")
		done <- true
	}()

	go func() {
		ctx.Log.Info("Calculating hash of stream...")
		hash, hashErr = util.GetSha256HashOfStream(ioutil.NopCloser(bytes.NewBuffer(b)))
		ctx.Log.Info("Hash of file is ", hash)
		done <- true
	}()

	for c := 0; c < 2; c++ {
		<-done
	}

	if hashErr != nil {
		return nil, hashErr
	}
	if writeErr != nil {
		return nil, writeErr
	}

	return &types.ObjectInfo{
		Location:   "ipfs/" + cid,
		Sha256Hash: hash,
		SizeBytes:  int64(len(b)),
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
