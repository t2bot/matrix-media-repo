package ipfs_proxy

import (
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy/ipfs_local"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy/ipfs_models"
)

var implementation IPFSImplementation

func Reload() {
	if implementation != nil {
		implementation.Stop()
	}
	implementation = nil

	if !config.Get().Features.IPFS.Enabled {
		return
	}

	if config.Get().Features.IPFS.Daemon {
		logrus.Info("Starting up local IPFS daemon...")
		// TODO: Make this work. It currently causes build errors.
		//impl, err := ipfs_embedded.NewEmbeddedIPFSNode()
		//if err != nil {
		//	panic(err)
		//}
		//implementation = impl
	} else {
		logrus.Info("Using localhost IPFS HTTP agent...")
		impl, err := ipfs_local.NewLocalIPFSImplementation()
		if err != nil {
			panic(err)
		}
		implementation = impl
	}
}

func Stop() {
	if implementation != nil {
		implementation.Stop()
	}
	implementation = nil
}

func getImpl() IPFSImplementation {
	if implementation == nil {
		Reload()
	}
	if implementation == nil {
		panic("missing ipfs implementation object")
	}
	return implementation
}

func GetObject(contentId string, ctx rcontext.RequestContext) (*ipfs_models.IPFSObject, error) {
	return getImpl().GetObject(contentId, ctx)
}
