package test

import (
	"context"
	"fmt"
	"log"
	"path"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ContainerDeps struct {
	ctx            context.Context
	pgContainer    *postgres.PostgresContainer
	redisContainer testcontainers.Container
	homeservers    []*SynapseDep
	machines       []*mmrContainer
}

func MakeTestDeps() (*ContainerDeps, error) {
	ctx := context.Background()

	// Start two synapses for testing
	syn1, err := MakeSynapse("first.example.org")
	if err != nil {
		return nil, err
	}
	syn2, err := MakeSynapse("second.example.org")
	if err != nil {
		return nil, err
	}

	// Start postgresql database
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/library/postgres:14"),
		postgres.WithDatabase("mmr"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test1234"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}
	pgConnStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, err
	}

	// Start a redis container
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "docker.io/library/redis:7",
			ExposedPorts: []string{"6379/tcp"},
			Mounts: []testcontainers.ContainerMount{
				testcontainers.BindMount(path.Join(".", "dev", "redis.conf"), "/usr/local/etc/redis/redis.conf"),
			},
			Cmd: []string{"redis-server", "/usr/local/etc/redis/redis.conf"},
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}
	redisHost, err := redisContainer.Host(ctx)
	if err != nil {
		return nil, err
	}
	redisPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	if err != nil {
		return nil, err
	}

	// Start two MMRs for testing
	mmrs, err := makeMmrInstances(ctx, 2, mmrTmplArgs{
		Homeservers: []mmrHomeserverTmplArgs{
			{
				ServerName:         syn1.ServerName,
				ClientServerApiUrl: syn1.ClientServerApiUrl,
			},
			{
				ServerName:         syn2.ServerName,
				ClientServerApiUrl: syn2.ClientServerApiUrl,
			},
		},
		RedisAddr:          fmt.Sprintf("%s:%d", redisHost, redisPort.Int()),
		PgConnectionString: pgConnStr,
	})

	return &ContainerDeps{
		ctx:            ctx,
		pgContainer:    pgContainer,
		redisContainer: redisContainer,
		homeservers:    []*SynapseDep{syn1, syn2},
		machines:       mmrs,
	}, nil
}

func (c *ContainerDeps) Teardown() {
	for _, mmr := range c.machines {
		mmr.Teardown()
	}
	for _, hs := range c.homeservers {
		hs.Teardown()
	}
	if err := c.redisContainer.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down redis container: %s", err.Error())
	}
	if err := c.pgContainer.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down mmr-postgres container: %s", err.Error())
	}
}
