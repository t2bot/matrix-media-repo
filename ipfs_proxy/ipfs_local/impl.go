package ipfs_local

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/ipfs/go-cid"
	httpapi "github.com/ipfs/go-ipfs-api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy/ipfs_models"
	"github.com/turt2live/matrix-media-repo/util"
)

type IPFSLocal struct {
	client *httpapi.Shell
}

func NewLocalIPFSImplementation() (IPFSLocal, error) {
	client := httpapi.NewLocalShell()
	return IPFSLocal{
		client: client,
	}, nil
}

func (i IPFSLocal) GetObject(contentId string, ctx rcontext.RequestContext) (*ipfs_models.IPFSObject, error) {
	ipfsCid, err := cid.Decode(contentId)
	if err != nil {
		return nil, err
	}

	c, err := i.client.Cat(ipfsCid.String())
	if err != nil {
		return nil, err
	}
	defer c.Close()

	b, err := ioutil.ReadAll(c)
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
	p, err := i.client.Add(data)
	if err != nil {
		return "", err
	}
	return p, nil
}

func (i IPFSLocal) Stop() {
	// Nothing to do
}
