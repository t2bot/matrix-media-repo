package ipfs_local

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy/ipfs_models"
	"github.com/turt2live/matrix-media-repo/util"
)

type IPFSLocal struct {
	client *httpapi.HttpApi
}

func NewLocalIPFSImplementation() (IPFSLocal, error) {
	client, err := httpapi.NewLocalApi()
	return IPFSLocal{
		client: client,
	}, err
}

func (i IPFSLocal) GetObject(contentId string, ctx rcontext.RequestContext) (*ipfs_models.IPFSObject, error) {
	ipfsCid, err := cid.Decode(contentId)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()

	ipfsPath := path.IpfsPath(ipfsCid)
	r, err := i.client.Object().Data(timeoutCtx, ipfsPath)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return &ipfs_models.IPFSObject{
		ContentType: "application/octet-stream", // TODO: Actually fetch
		FileName:    "ipfs.dat",                 // TODO: Actually fetch
		SizeBytes:   int64(len(b)),
		Data:        util.BufferToStream(bytes.NewBuffer(b)), // stream to avoid log spam
	}, nil
}

func (i IPFSLocal) PutObject(data io.Reader, ctx rcontext.RequestContext) (string, error) {
	ipfsFile := files.NewReaderFile(data)
	p, err := i.client.Unixfs().Add(ctx.Context, ipfsFile)
	if err != nil {
		return "", err
	}
	return p.Cid().String(), nil
}

func (i IPFSLocal) Stop() {
	// Nothing to do
}
