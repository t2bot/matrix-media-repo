package test_internals

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
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
	S3Endpoint         string
}

type mmrContainer struct {
	ctx           context.Context
	container     testcontainers.Container
	tmpConfigPath string

	HttpUrl   string
	MachineId int
}

var mmrCachedImage string
var mmrCachedContext *os.File
var mmrCachedSuites atomic.Int32

func MarkUsingCachedMmrBuild() {
	mmrCachedSuites.Add(1)
	log.Println("+1 cached build usage")
}

func reuseMmrBuild(ctx context.Context) (string, error) {
	if mmrCachedImage != "" {
		return mmrCachedImage, nil
	}
	log.Println("[Test Deps] Building MMR image...")
	cr, err := createDockerContext()
	if err != nil {
		return "", err
	}
	mmrCachedContext = cr
	buildReq := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile:     "Dockerfile",
				Context:        ".",
				ContextArchive: cr,
				PrintBuildLog:  true,
			},
		},
		Started: false,
	}
	provider, err := buildReq.ProviderType.GetProvider(testcontainers.WithLogger(testcontainers.Logger))
	if err != nil {
		return "", err
	}
	if dockerProvider, ok := provider.(*testcontainers.DockerProvider); !ok {
		return "", errors.New("expected a docker provider")
	} else {
		tag, err := dockerProvider.BuildImage(ctx, &buildReq)
		if err != nil {
			return "", err
		}
		mmrCachedImage = tag
	}
	log.Println("[Test Deps] Cached build as ", mmrCachedImage)
	return mmrCachedImage, nil
}

func writeMmrConfig(tmplArgs mmrTmplArgs) (string, error) {
	// Prepare a config template
	t, err := template.New("mmr.config.yaml").ParseFiles(path.Join(".", "test", "templates", "mmr.config.yaml"))
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
	intTmpName, err := writeMmrConfig(tmplArgs)
	if err != nil {
		return nil, err
	}

	// Cache the MMR image
	mmrImage, err := reuseMmrBuild(ctx)
	if err != nil {
		return nil, err
	}

	// Start the containers (using the same DB and config)
	mmrs := make([]*mmrContainer, 0)
	for i := 0; i < count; i++ {
		// Create the docker container (from dockerfile)
		p, _ := nat.NewPort("tcp", "8000")
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        mmrImage,
				ExposedPorts: []string{"8000/tcp"},
				Mounts: []testcontainers.ContainerMount{
					testcontainers.BindMount(intTmpName, "/data/media-repo.yaml"),
				},
				Env: map[string]string{
					"MACHINE_ID": strconv.Itoa(i),
				},
				Networks:   []string{depNet.NetId},
				WaitingFor: wait.ForHTTP("/healthz").WithPort(p),
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

func TeardownMmrCaches() {
	mmrCachedSuites.Add(-1)
	if mmrCachedSuites.Load() > 0 {
		log.Println("Not cleaning up MMR cached images: image still in use")
		return
	}

	if mmrCachedContext != nil {
		_ = mmrCachedContext.Close() // ignore errors because testcontainers might have already closed it
		if err := os.Remove(mmrCachedContext.Name()); err != nil && !os.IsNotExist(err) {
			log.Fatalf("Error cleaning up MMR cached context file '%s': %s", mmrCachedContext.Name(), err.Error())
		}
	}
	if mmrCachedImage != "" {
		if p, err := (testcontainers.GenericContainerRequest{}.ProviderType.GetProvider()); err != nil {
			log.Fatalf("Error cleaning up MMR cached build image '%s': %s", mmrCachedImage, err.Error())
		} else if dockerProvider, ok := p.(*testcontainers.DockerProvider); !ok {
			log.Fatalf("Error cleaning up MMR cached build image '%s': unable to cast provider to DockerProvider", mmrCachedImage)
		} else {
			rmImage := func(imageName string) {
				if _, err = dockerProvider.Client().ImageRemove(context.Background(), imageName, types.ImageRemoveOptions{
					PruneChildren: true,
				}); err != nil {
					log.Fatalf("Error removing MMR cached build image '%s': %s", imageName, err.Error())
				}
			}
			if images, err := dockerProvider.Client().ImageList(context.Background(), types.ImageListOptions{All: true}); err != nil {
				log.Fatalf("Error listing Docker images to clean up MMR image '%s': %s", mmrCachedImage, err.Error())
			} else {
				rmImage(mmrCachedImage)
				for _, i := range images {
					if i.Labels != nil && i.Labels["io.t2bot.mmr.cleanup"] == "true" {
						rmImage(i.ID)
					}
				}
			}
		}
	}
}
