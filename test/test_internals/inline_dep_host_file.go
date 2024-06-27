package test_internals

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type HostedFile struct {
	upstream          *ContainerDeps
	nginx             testcontainers.Container
	tempDirectoryPath string

	PublicUrl      string
	PublicHostname string
}

func ServeFile(fileName string, deps *ContainerDeps, contents string) (*HostedFile, error) {
	container, writeFn, err := LazyServeFile(fileName, deps)
	if writeFn != nil {
		err2 := writeFn(contents)
		if err2 != nil {
			return nil, err2
		}
	}
	return container, err
}

func LazyServeFile(fileName string, deps *ContainerDeps) (*HostedFile, func(string) error, error) {
	tmp, err := os.MkdirTemp(os.TempDir(), "mmr-nginx")
	if err != nil {
		return nil, nil, err
	}

	err = os.Chmod(tmp, 0755)
	if err != nil {
		return nil, nil, err
	}

	err = os.MkdirAll(path.Join(tmp, path.Dir(fileName)), 0755)
	if err != nil {
		return nil, nil, err
	}

	writeFn := func(contents string) error {
		f, err := os.Create(path.Join(tmp, fileName))
		if err != nil {
			return err
		}
		defer func(f *os.File) {
			_ = f.Close()
		}(f)

		_, err = f.Write([]byte(contents))
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}

		return nil
	}

	nginx, err := testcontainers.GenericContainer(deps.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "docker.io/library/nginx:latest",
			ExposedPorts: []string{"80/tcp"},
			Mounts: []testcontainers.ContainerMount{
				testcontainers.BindMount(tmp, "/usr/share/nginx/html"),
			},
			Networks:   []string{deps.depNet.NetId},
			WaitingFor: wait.ForListeningPort("80/tcp"),
		},
		Started: true,
	})
	if err != nil {
		return nil, nil, err
	}

	nginxIp, err := nginx.ContainerIP(deps.ctx)
	if err != nil {
		return nil, nil, err
	}

	//goland:noinspection HttpUrlsUsage
	return &HostedFile{
		upstream:          deps,
		nginx:             nginx,
		tempDirectoryPath: tmp,
		PublicUrl:         fmt.Sprintf("http://%s:%d/%s", nginxIp, 80, fileName),
		PublicHostname:    fmt.Sprintf("%s:%d", nginxIp, 80),
	}, writeFn, nil
}

func (f *HostedFile) Teardown() {
	if err := f.nginx.Terminate(f.upstream.ctx); err != nil {
		log.Fatalf("Error shutting down nginx container: %s", err.Error())
	}
	if err := os.RemoveAll(f.tempDirectoryPath); err != nil {
		log.Fatalf("Error cleaning up temporarily hosted file: %s", err.Error())
	}
}

func (f *HostedFile) Logs() (io.ReadCloser, error) {
	return f.nginx.Logs(context.Background())
}
