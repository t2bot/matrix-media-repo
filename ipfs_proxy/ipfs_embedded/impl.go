package ipfs_embedded

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	icore "github.com/ipfs/interface-go-ipfs-core"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	ipfsConfig "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/bootstrap"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy/ipfs_models"
	"github.com/turt2live/matrix-media-repo/util"
)

type IPFSEmbedded struct {
	api         icore.CoreAPI
	node        *core.IpfsNode
	ctx         context.Context
	cancelCtxFn context.CancelFunc
}

func NewEmbeddedIPFSNode() (IPFSEmbedded, error) {
	// Startup routine modified from:
	// https://github.com/ipfs/kubo/blob/083ef47ce84a5bd9a93f0ce0afaf668881dc1f35/docs/examples/go-ipfs-as-a-library/main.go

	basePath := config.Get().Features.IPFS.Daemon.RepoPath
	dataPath := path.Join(basePath, "data")

	ctx, cancel := context.WithCancel(context.Background())

	blank := IPFSEmbedded{}

	// Load plugins
	logrus.Info("Loading plugins for IPFS embedded node...")
	plugins, err := loader.NewPluginLoader(filepath.Join(basePath, "plugins"))
	if err != nil {
		cancel()
		return blank, err
	}
	err = plugins.Initialize()
	if err != nil {
		cancel()
		return blank, err
	}
	err = plugins.Inject()
	if err != nil {
		cancel()
		return blank, err
	}

	logrus.Info("Generating config for IPFS embedded node")
	cfg, err := ipfsConfig.Init(ioutil.Discard, 2048)
	if err != nil {
		cancel()
		return blank, err
	}

	logrus.Info("Initializing IPFS embedded node")
	err = fsrepo.Init(dataPath, cfg)
	if err != nil {
		cancel()
		return blank, err
	}

	logrus.Info("Starting fsrepo for IPFS embedded node")
	repo, err := fsrepo.Open(dataPath)
	if err != nil {
		cancel()
		return blank, err
	}

	// Create the node from the repo
	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption,
		Repo:    repo,
	}

	logrus.Info("Building IPFS embedded node")
	node, err := core.NewNode(ctx, nodeOptions)
	if err != nil {
		cancel()
		return blank, err
	}

	logrus.Info("Generating API reference for IPFS embedded node")
	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		cancel()
		return blank, err
	}

	// Connect to peers so we can actually get started
	logrus.Info("Connecting to peers for IPFS embedded node")
	err = node.Bootstrap(bootstrap.DefaultBootstrapConfig)
	if err != nil {
		cancel()
		return blank, err
	}

	logrus.Info("Done building IPFS embedded node")
	return IPFSEmbedded{
		api:         api,
		node:        node,
		ctx:         ctx,
		cancelCtxFn: cancel,
	}, nil
}

func (i IPFSEmbedded) GetObject(contentId string, ctx rcontext.RequestContext) (*ipfs_models.IPFSObject, error) {
	ctx.Log.Info("Getting object from embedded IPFS node")
	ipfsCid, err := cid.Decode(contentId)
	if err != nil {
		return nil, err
	}

	ctx.Log.Info("Resolving path and node")
	timeoutCtx, cancel := context.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()
	ipfsPath := icorepath.IpfsPath(ipfsCid)
	r, err := i.api.Object().Data(timeoutCtx, ipfsPath)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	ctx.Log.Info("Returning object")
	return &ipfs_models.IPFSObject{
		ContentType: "application/octet-stream", // TODO: Actually fetch
		FileName:    "ipfs.dat",                 // TODO: Actually fetch
		SizeBytes:   int64(len(b)),
		Data:        util.BufferToStream(bytes.NewBuffer(b)), // stream to avoid log spam
	}, nil
}

func (i IPFSEmbedded) PutObject(data io.Reader, ctx rcontext.RequestContext) (string, error) {
	ipfsFile := files.NewReaderFile(data)
	p, err := i.api.Unixfs().Add(ctx.Context, ipfsFile)
	if err != nil {
		return "", err
	}
	return p.Cid().String(), nil
}

func (i IPFSEmbedded) Stop() {
	i.cancelCtxFn()
	_ = i.node.Close()
}
