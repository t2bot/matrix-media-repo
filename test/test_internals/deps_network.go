package test_internals

import (
	"context"
	"log"

	"github.com/t2bot/matrix-media-repo/util/ids"
	"github.com/testcontainers/testcontainers-go"
)

type NetworkDep struct {
	ctx       context.Context
	dockerNet testcontainers.Network

	NetId string
}

type netCustomizer struct {
	testcontainers.ContainerCustomizer
	network *NetworkDep
}

func (c *netCustomizer) Customize(req *testcontainers.GenericContainerRequest) error {
	if req.Networks == nil {
		req.Networks = make([]string, 0)
	}
	req.Networks = append(req.Networks, c.network.NetId)
	return nil
}

func MakeNetwork() (*NetworkDep, error) {
	ctx := context.Background()

	netId, err := ids.NewUniqueId()
	if err != nil {
		return nil, err
	}
	dockerNet, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: netId,
		},
		ProviderType: testcontainers.ProviderDocker,
	})
	if err != nil {
		return nil, err
	}

	return &NetworkDep{
		ctx:       ctx,
		dockerNet: dockerNet,
		NetId:     netId,
	}, nil
}

func (n *NetworkDep) ApplyToContainer() testcontainers.ContainerCustomizer {
	return &netCustomizer{network: n}
}

func (n *NetworkDep) Teardown() {
	if err := n.dockerNet.Remove(n.ctx); err != nil {
		log.Fatalf("Error cleaning up docker network '%s': %s", n.NetId, err.Error())
	}
}
