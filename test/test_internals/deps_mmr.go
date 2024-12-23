package test_internals

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type mmrHomeserverTmplArgs struct {
	ServerName         string
	ClientServerApiUrl string
	SigningKeyPath     string
	PublicBaseUrl      string
}

type mmrTmplArgs struct {
	Homeservers        []*mmrHomeserverTmplArgs
	RedisAddr          string
	PgConnectionString string
	S3Endpoint         string
}

type mmrContainer struct {
	ctx           context.Context
	container     testcontainers.Container
	tmpConfigPath string

	HttpUrl   string
	MachineId int
}

// call this function from the `test` package, as it contains file paths relative to that package.
func writeMmrConfig(tmplArgs mmrTmplArgs) (string, error) {
	// Prepare a config template
	t, err := template.New("mmr.config.yaml").ParseFiles(filepath.Join(".", "templates", "mmr.config.yaml"))
	if err != nil {
		return "", err
	}
	w := new(strings.Builder)
	err = t.Execute(w, tmplArgs)
	if err != nil {
		return "", err
	}

	// Write the MMR config to a temp file
	f, err := os.CreateTemp(os.TempDir(), "mmr-tests-mediarepo")
	if err != nil {
		return "", err
	}
	err = f.Chmod(0644)
	if err != nil {
		return "", err
	}
	_, err = f.Write([]byte(w.String()))
	if err != nil {
		return "", err
	}
	err = f.Close()
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

func makeMmrInstances(ctx context.Context, count int, depNet *NetworkDep, tmplArgs mmrTmplArgs) ([]*mmrContainer, error) {
	// We need to relocate the signing key paths for a Docker mount
	additionalFiles := make([]testcontainers.ContainerFile, 0)
	for i, hs := range tmplArgs.Homeservers {
		if hs.SigningKeyPath != "" {
			inContainerName := fmt.Sprintf("/data/hs%d.key", i)
			additionalFiles = append(additionalFiles, testcontainers.ContainerFile{
				HostFilePath:      hs.SigningKeyPath,
				ContainerFilePath: inContainerName,
				FileMode:          0o0777,
			})

			hs.SigningKeyPath = inContainerName
		}
	}

	// ... then we can write the config and get the temp file path for it
	intTmpName, err := writeMmrConfig(tmplArgs)
	if err != nil {
		return nil, err
	}

	// Start the containers (using the same DB and config)
	mmrs := make([]*mmrContainer, 0)
	for i := 0; i < count; i++ {

		p, _ := nat.NewPort("tcp", "8000")
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:       filepath.Join(".."),
					Dockerfile:    "Dockerfile",
					PrintBuildLog: true,
					KeepImage:     true,
				},
				ExposedPorts: []string{"8000/tcp"},
				Files: append(additionalFiles, testcontainers.ContainerFile{
					HostFilePath:      intTmpName,
					ContainerFilePath: "/data/media-repo.yaml",
				}),
				Env: map[string]string{
					"MACHINE_ID":                      strconv.Itoa(i),
					"MEDIA_REPO_HTTP_ONLY_FEDERATION": "true",
				},
				Networks:   []string{depNet.NetId},
				WaitingFor: wait.ForHTTP("/healthz").WithPort(p),
				HostConfigModifier: func(c *container.HostConfig) {
					c.ExtraHosts = append(c.ExtraHosts, "host.docker.internal:host-gateway")
				},
			},
			Started: true,
		})
		if err != nil {
			return nil, err
		}

		// Get the http url
		mmrHost, err := container.Host(ctx)
		if err != nil {
			return nil, err
		}
		mmrPort, err := container.MappedPort(ctx, "8000/tcp")
		if err != nil {
			return nil, err
		}
		//goland:noinspection HttpUrlsUsage
		csApiUrl := fmt.Sprintf("http://%s:%d", mmrHost, mmrPort.Int())

		// Create the container object
		mmrs = append(mmrs, &mmrContainer{
			ctx:           ctx,
			container:     container,
			tmpConfigPath: intTmpName,
			HttpUrl:       csApiUrl,
			MachineId:     i,
		})
	}

	return mmrs, nil
}

func (c *mmrContainer) Teardown() {
	if err := c.container.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down MMR machine %d: %s", c.MachineId, err.Error())
	}
	if err := os.Remove(c.tmpConfigPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error cleaning up MMR config file '%s': %s", c.tmpConfigPath, err.Error())
	}
}

func (c *mmrContainer) Logs() (io.ReadCloser, error) {
	return c.container.Logs(c.ctx)
}
