package test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type mmrHomeserverTmplArgs struct {
	ServerName         string
	ClientServerApiUrl string
}

type mmrTmplArgs struct {
	Homeservers        []mmrHomeserverTmplArgs
	RedisAddr          string
	PgConnectionString string
}

type mmrContainer struct {
	ctx           context.Context
	container     testcontainers.Container
	tmpConfigPath string

	HttpUrl   string
	MachineId int
}

func makeMmrInstances(ctx context.Context, count int, tmplArgs mmrTmplArgs) ([]*mmrContainer, error) {
	// Prepare a config template
	t, err := template.New("mmr.config.yaml").ParseFiles(path.Join(".", "test", "templates", "mmr.config.yaml"))
	if err != nil {
		return nil, err
	}
	w := new(strings.Builder)
	err = t.Execute(w, tmplArgs)
	if err != nil {
		return nil, err
	}

	// Write the MMR config to a temp file
	f, err := os.CreateTemp(os.TempDir(), "mmr-tests-mediarepo")
	if err != nil {
		return nil, err
	}
	_, err = f.Write([]byte(w.String()))
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	// Start the containers (using the same DB and config)
	mmrs := make([]*mmrContainer, 0)
	for i := 0; i < count; i++ {
		// Create the docker container (from dockerfile)
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Dockerfile: "Dockerfile",
				},
				ExposedPorts: []string{"8000/tcp"},
				Mounts: []testcontainers.ContainerMount{
					testcontainers.BindMount(f.Name(), "/data/media-repo.yaml"),
				},
				Env: map[string]string{
					"MACHINE_ID": strconv.Itoa(i),
				},
				WaitingFor: wait.ForHTTP("/healthz"),
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
			tmpConfigPath: f.Name(),
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
