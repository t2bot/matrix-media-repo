package test_internals

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

type minioTmplArgs struct {
	ConsoleAddress string
}

type MinioDep struct {
	ctx       context.Context
	container *minio.MinioContainer

	Endpoint         string
	ExternalEndpoint string
}

func MakeMinio(depNet *NetworkDep) (*MinioDep, error) {
	ctx := context.Background()

	// Start the minio container
	container, err := minio.Run(ctx,
		"quay.io/minio/minio:latest",
		minio.WithPassword("test1234"),
		minio.WithUsername("admin"),
		tcnetwork.WithNetwork([]string{"minio"}, depNet.dockerNet),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				ExposedPorts: []string{"9090/tcp"},
				Cmd:          []string{"--console-address", ":9090"},
				Files: []testcontainers.ContainerFile{
					{
						HostFilePath:      filepath.Join("testdata", "function.zip"),
						ContainerFilePath: "/tmp/function.zip",
					},
				},
			},
		}),
	)
	if err != nil {
		return nil, err
	}

	if _, _, err = container.Exec(ctx, []string{"mkdir", "-p", "/data"}); err != nil {
		return nil, err
	}

	// Find the minio connection details
	minioIp, err := container.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}
	minioHost, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}
	minioPort, err := container.MappedPort(ctx, "9090/tcp")
	if err != nil {
		return nil, err
	}

	// Prepare the test script
	t, err := template.New("minio-config.sh").ParseFiles(path.Join(".", "test", "templates", "minio-config.sh"))
	if err != nil {
		return nil, err
	}
	w := new(strings.Builder)
	//goland:noinspection HttpUrlsUsage
	err = t.Execute(w, minioTmplArgs{
		ConsoleAddress: fmt.Sprintf("http://%s:%d", minioIp, 9000), // we're behind the network
	})
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(os.TempDir(), "mmr-minio")
	if err != nil {
		return nil, err
	}
	err = f.Chmod(0644)
	if err != nil {
		return nil, err
	}
	_, err = f.Write([]byte(strings.ReplaceAll(w.String(), "\r\n", "\n"))) // dos2unix now instead of in the container
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	// Use an intermediary container to set up the minio instance
	_, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "docker.io/alpine:3.18",
			Networks:   []string{depNet.NetId},
			WaitingFor: wait.ForLog("This line marks WaitFor as done").WithStartupTimeout(120 * time.Second),
			Mounts: []testcontainers.ContainerMount{
				testcontainers.BindMount(f.Name(), "/run.sh"),
			},
			Cmd: []string{"ash", "-c", "chmod +x /run.sh && /run.sh"},
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}

	return &MinioDep{
		ctx:              ctx,
		container:        container,
		Endpoint:         fmt.Sprintf("%s:%d", minioIp, 9000), // we're behind the network
		ExternalEndpoint: fmt.Sprintf("%s:%d", minioHost, minioPort.Int()),
	}, nil
}

func (c *MinioDep) Teardown() {
	if err := c.container.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down minio: %s", err.Error())
	}
}
