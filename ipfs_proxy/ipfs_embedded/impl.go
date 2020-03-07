package ipfs_embedded

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	ipfsConfig "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multiaddr"
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
	// https://github.com/ipfs/go-ipfs/blob/083ef47ce84a5bd9a93f0ce0afaf668881dc1f35/docs/examples/go-ipfs-as-a-library/main.go

	basePath := config.Get().Features.IPFS.Daemon.RepoPath
	dataPath := path.Join(basePath, "data")

	ctx, cancel := context.WithCancel(context.Background())

	blank := IPFSEmbedded{}

	// Load plugins
	logrus.Info("Loading plugins for IPFS embedded node...")
	plugins, err := loader.NewPluginLoader(filepath.Join(basePath, "plugins"))
	if err != nil {
		return blank, err
	}
	err = plugins.Initialize()
	if err != nil {
		return blank, err
	}
	err = plugins.Inject()
	if err != nil {
		return blank, err
	}

	logrus.Info("Generating config for IPFS embedded node")
	cfg, err := ipfsConfig.Init(ioutil.Discard, 2048)
	if err != nil {
		return blank, err
	}

	logrus.Info("Initializing IPFS embedded node")
	err = fsrepo.Init(dataPath, cfg)
	if err != nil {
		return blank, err
	}

	logrus.Info("Starting fsrepo for IPFS embedded node")
	repo, err := fsrepo.Open(dataPath)
	if err != nil {
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
		return blank, err
	}

	logrus.Info("Generating API reference for IPFS embedded node")
	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return blank, err
	}

	// Connect to peers so we can actually get started
	logrus.Info("Connecting to peers for IPFS embedded node")
	bootstrapNodes := []string{
		// IPFS Bootstrapper nodes.
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

		// IPFS Cluster Pinning nodes
		"/ip4/138.201.67.219/tcp/4001/p2p/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",
		"/ip4/138.201.67.220/tcp/4001/p2p/QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i",
		"/ip4/138.201.68.74/tcp/4001/p2p/QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR",
		"/ip4/94.130.135.167/tcp/4001/p2p/QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE",
	}
	var wg sync.WaitGroup
	peerInfos := make(map[peer.ID]*peerstore.PeerInfo, len(bootstrapNodes))
	for _, addrStr := range bootstrapNodes {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			return blank, err
		}

		pii, err := peerstore.InfoFromP2pAddr(addr)
		if err != nil {
			return blank, err
		}

		pi, ok := peerInfos[pii.ID]
		if !ok {
			pi = &peerstore.PeerInfo{ID: pii.ID}
			peerInfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}
	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peerstore.PeerInfo) {
			defer wg.Done()
			err := api.Swarm().Connect(ctx, *peerInfo)
			if err != nil {
				logrus.Error(err)
			} else {
				logrus.Infof("Connected to %s as a peer", peerInfo.String())
			}
		}(peerInfo)
	}
	wg.Wait()

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
	node, err := i.api.ResolveNode(timeoutCtx, ipfsPath)
	if err != nil {
		return nil, err
	}

	ctx.Log.Info("Returning object")
	return &ipfs_models.IPFSObject{
		ContentType: "application/octet-stream", // TODO: Actually fetch
		FileName:    "ipfs.dat",                 // TODO: Actually fetch
		SizeBytes:   int64(len(node.RawData())),
		Data:        util.BufferToStream(bytes.NewBuffer(node.RawData())), // stream to avoid log spam
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
	i.node.Close()
}
