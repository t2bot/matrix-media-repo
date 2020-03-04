package ipfs_embedded

import (
	"bytes"
	"context"
	"io/ioutil"
	"sync"

	"github.com/ipfs/go-cid"
	ipfsConfig "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	icorepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy/ipfs_models"
	"github.com/turt2live/matrix-media-repo/util"
)

type IPFSEmbedded struct {
	api  icore.CoreAPI
	node *core.IpfsNode
}

func NewEmbeddedIPFSNode() (IPFSEmbedded, error) {
	// Startup routine modified from:
	// https://github.com/ipfs/go-ipfs/blob/083ef47ce84a5bd9a93f0ce0afaf668881dc1f35/docs/examples/go-ipfs-as-a-library/main.go

	blank := IPFSEmbedded{}

	// Create the repo (in ephemeral space)
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return blank, err
	}

	cfg, err := ipfsConfig.Init(ioutil.Discard, 2048)
	if err != nil {
		return blank, err
	}

	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return blank, err
	}

	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return blank, err
	}

	// Create the node from the repo
	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption,
		Repo:    repo,
	}

	node, err := core.NewNode(context.Background(), nodeOptions)
	if err != nil {
		return blank, err
	}

	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return blank, err
	}

	// Connect to peers so we can actually get started
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
			err := api.Swarm().Connect(context.Background(), *peerInfo)
			if err != nil {
				logrus.Error(err)
			}
		}(peerInfo)
	}
	wg.Wait()

	return IPFSEmbedded{
		api:  api,
		node: node,
	}, nil
}

func (i IPFSEmbedded) GetObject(contentId string, ctx rcontext.RequestContext) (*ipfs_models.IPFSObject, error) {
	ipfsCid, err := cid.Decode(contentId)
	if err != nil {
		return nil, err
	}

	ipfsPath := icorepath.IpfsPath(ipfsCid)
	node, err := i.api.ResolveNode(ctx.Context, ipfsPath)
	if err != nil {
		return nil, err
	}

	return &ipfs_models.IPFSObject{
		ContentType: "application/octet-stream", // TODO: Actually fetch
		FileName:    "ipfs.dat",                 // TODO: Actually fetch
		SizeBytes:   int64(len(node.RawData())),
		Data:        util.BufferToStream(bytes.NewBuffer(node.RawData())), // stream to avoid log spam
	}, nil
}

func (i IPFSEmbedded) Stop() {
	i.node.Close()
}
