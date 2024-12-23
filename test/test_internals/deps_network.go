package test_internals

import (
	"context"
	"log"

	"github.com/testcontainers/testcontainers-go"
)

type NetworkDep struct {
	ctx       context.Context
	dockerNet *testcontainers.DockerNetwork

	NetId string
}

func (n *NetworkDep) Teardown() {
	if err := n.dockerNet.Remove(n.ctx); err != nil {
		log.Fatalf("Error cleaning up docker network '%s': %s", n.NetId, err.Error())
	}
}
