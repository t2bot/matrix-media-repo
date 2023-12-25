package test_internals

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type minioTmplArgs struct {
	ConsoleAddress string
}

type MinioDep struct {
	ctx       context.Context
	container testcontainers.Container

	Endpoint         string
	ExternalEndpoint string
}

func MakeMinio(depNet *NetworkDep) (*MinioDep, error) {
	ctx := context.Background()

	// Start the minio container
	consolePort, _ := nat.NewPort("tcp", "9090")
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "quay.io/minio/minio:latest",
			ExposedPorts: []string{"9000/tcp", "9090/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "admin",
				"MINIO_ROOT_PASSWORD": "test1234",
			},
			WaitingFor: wait.ForHTTP("/login").WithPort(consolePort),
			Networks:   []string{depNet.NetId},
			Cmd:        []string{"server", "/data", "--console-address", ":9090"},
			// we don't bind any volumes because we don't care if we lose the data
		},
		Started: true,
	})
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
			Files: []testcontainers.ContainerFile{
				{ContainerFilePath: "./run.sh", HostFilePath: f.Name()},
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
