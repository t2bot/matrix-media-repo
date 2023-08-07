package test

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
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type synapseTmplArgs struct {
	ServerName string
	PgHost     string
	PgPort     int
}

type SynapseDep struct {
	ctx           context.Context
	pgContainer   *postgres.PostgresContainer
	synContainer  testcontainers.Container
	tmpConfigPath string

	ClientServerApiUrl string
	ServerName         string
}

func MakeSynapse(domainName string) (*SynapseDep, error) {
	ctx := context.Background()

	// Start postgresql database
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/library/postgres:14"),
		postgres.WithDatabase("synapse"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test1234"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	// Prepare the synapse config
	t, err := template.New("synapse.homeserver.yaml").ParseFiles(path.Join(".", "test", "templates", "synapse.homeserver.yaml"))
	if err != nil {
		return nil, err
	}
	pghost, err := pgContainer.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}
	pgport, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return nil, err
	}
	w := new(strings.Builder)
	err = t.Execute(w, synapseTmplArgs{
		ServerName: domainName,
		PgHost:     pghost,
		PgPort:     pgport.Int(),
	})
	if err != nil {
		return nil, err
	}

	// Write the synapse config to a temp file
	f, err := os.CreateTemp(os.TempDir(), "mmr-tests-synapse")
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

	// Start synapse
	p, _ := nat.NewPort("tcp", "8008")
	d, err := os.MkdirTemp(os.TempDir(), "mmr-synapse")
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	synContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "docker.io/matrixdotorg/synapse:v1.89.0",
			ExposedPorts: []string{"8008/tcp"},
			Mounts: []testcontainers.ContainerMount{
				testcontainers.BindMount(f.Name(), "/data/homeserver.yaml"),
				testcontainers.BindMount(path.Join(cwd, ".", "test", "templates", "synapse.log.config"), "/data/log.config"),
				testcontainers.BindMount(d, "/app"),
			},
			WaitingFor: wait.ForHTTP("/health").WithPort(p),
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}

	// Prepare the CS API URL
	synHost, err := synContainer.Host(ctx)
	if err != nil {
		return nil, err
	}
	synPort, err := synContainer.MappedPort(ctx, "8008/tcp")
	if err != nil {
		return nil, err
	}
	//goland:noinspection HttpUrlsUsage
	csApiUrl := fmt.Sprintf("http://%s:%d", synHost, synPort.Int())

	// Create the dependency
	return &SynapseDep{
		ctx:                ctx,
		pgContainer:        pgContainer,
		synContainer:       synContainer,
		tmpConfigPath:      f.Name(),
		ClientServerApiUrl: csApiUrl,
		ServerName:         domainName,
	}, nil
}

func (c *SynapseDep) Teardown() {
	if err := c.synContainer.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down synapse container for '%s': %s", c.ServerName, err.Error())
	}
	if err := c.pgContainer.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down synapse-postgres container for '%s': %s", c.ServerName, err.Error())
	}
	if err := os.Remove(c.tmpConfigPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error cleaning up synapse config file '%s': %s", c.tmpConfigPath, err.Error())
	}
}
