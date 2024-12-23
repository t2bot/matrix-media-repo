package test_internals

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/t2bot/matrix-media-repo/common/assets"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/synapse"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ContainerDeps struct {
	ctx               context.Context
	pgContainer       *postgres.PostgresContainer
	redisContainer    testcontainers.Container
	minioDep          *MinioDep
	depNet            *NetworkDep
	mmrExtConfigPath  string
	mmrSigningKeyPath string

	Homeservers []*SynapseDep
	Machines    []*mmrContainer
}

func MakeTestDeps() (*ContainerDeps, error) {
	ctx := context.Background()

	// Create a network
	depNet, err := MakeNetwork()
	if err != nil {
		return nil, err
	}

	// Create a shared signing key for the MMR instances
	signingKeyFile, err := os.CreateTemp(os.TempDir(), "mmr-signing-key")
	if err != nil {
		return nil, err
	}
	signingKey, err := homeserver_interop.GenerateSigningKey()
	if err != nil {
		return nil, err
	}
	b, err := mmr.EncodeSigningKey(signingKey)
	if err != nil {
		return nil, err
	}
	_, err = signingKeyFile.Write(b)
	if err != nil {
		return nil, err
	}
	err = signingKeyFile.Close()
	if err != nil {
		return nil, err
	}

	// And use that same signing key for Synapse
	synapseSigningKeyFile, err := os.CreateTemp(os.TempDir(), "mmr-synapse-signing-key")
	if err != nil {
		return nil, err
	}
	b, err = synapse.EncodeSigningKey(signingKey)
	if err != nil {
		return nil, err
	}
	_, err = synapseSigningKeyFile.Write(b)
	if err != nil {
		return nil, err
	}
	err = synapseSigningKeyFile.Close()
	if err != nil {
		return nil, err
	}
	err = os.Chmod(synapseSigningKeyFile.Name(), 0777) // XXX: Not great, but works.
	if err != nil {
		return nil, err
	}

	// Start two synapses for testing
	syn1, err := MakeSynapse("first.example.org", depNet, synapseSigningKeyFile.Name())
	if err != nil {
		return nil, err
	}
	syn2, err := MakeSynapse("second.example.org", depNet, synapseSigningKeyFile.Name())
	if err != nil {
		return nil, err
	}

	// Start postgresql database
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/library/postgres:16"),
		postgres.WithDatabase("mmr"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test1234"),
		depNet.ApplyToContainer(),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}
	pgHost, err := pgContainer.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}
	// we can hardcode the port and most of the connection details because we're behind the docker network here
	pgConnStr := fmt.Sprintf("host=%s port=5432 user=postgres password=test1234 dbname=mmr sslmode=disable", pgHost)
	// the external connection string is a bit harder, because testcontainers wants to use `localhost` as a hostname,
	// which prevents us from using the ConnectionString() function. We instead build the connection string manually
	//pgExtPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	extPgConnStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, err
	}
	//extPgConnStr := fmt.Sprintf("host=%s port=%d user=postgres password=test1234 dbname=mmr sslmode=disable", testcontainers.HostInternal, pgExtPort.Int())

	// Start a redis container
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "docker.io/library/redis:7",
			ExposedPorts: []string{"6379/tcp"},
			Mounts: []testcontainers.ContainerMount{
				testcontainers.BindMount(path.Join(cwd, ".", "dev", "redis.conf"), "/usr/local/etc/redis/redis.conf"),
			},
			Cmd:        []string{"redis-server", "/usr/local/etc/redis/redis.conf"},
			Networks:   []string{depNet.NetId},
			WaitingFor: wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}
	redisIp, err := redisContainer.ContainerIP(ctx)
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

	// Start a minio (s3) container
	minioDep, err := MakeMinio(depNet)
	if err != nil {
		return nil, err
	}

	// Start two MMRs for testing
	tmplArgs := mmrTmplArgs{
		Homeservers: []*mmrHomeserverTmplArgs{
			{
				ServerName:         syn1.ServerName,
				ClientServerApiUrl: syn1.InternalClientServerApiUrl,
				SigningKeyPath:     strings.ReplaceAll(signingKeyFile.Name(), "\\", "\\\\"),
			},
			{
				ServerName:         syn2.ServerName,
				ClientServerApiUrl: syn2.InternalClientServerApiUrl,
				SigningKeyPath:     strings.ReplaceAll(signingKeyFile.Name(), "\\", "\\\\"),
			},
		},
		RedisAddr:          fmt.Sprintf("%s:%d", redisIp, 6379), // we're behind the network for redis
		PgConnectionString: pgConnStr,
		S3Endpoint:         minioDep.Endpoint,
	}
	mmrs, err := makeMmrInstances(ctx, 2, depNet, tmplArgs)
	if err != nil {
		return nil, err
	}

	// Generate a config that's safe to use in tests, for inspecting state of the containers
	tmplArgs.RedisAddr = fmt.Sprintf("%s:%d", redisHost, redisPort.Int())
	tmplArgs.PgConnectionString = extPgConnStr
	tmplArgs.S3Endpoint = minioDep.ExternalEndpoint
	tmplArgs.Homeservers[0].ClientServerApiUrl = syn1.ExternalClientServerApiUrl
	tmplArgs.Homeservers[1].ClientServerApiUrl = syn2.ExternalClientServerApiUrl
	tmpPath, err := writeMmrConfig(tmplArgs)
	if err != nil {
		return nil, err
	}
	config.Path = tmpPath
	assets.SetupMigrations(config.DefaultMigrationsPath)
	assets.SetupTemplates(config.DefaultTemplatesPath)
	assets.SetupAssets(config.DefaultAssetsPath)

	return &ContainerDeps{
		ctx:               ctx,
		pgContainer:       pgContainer,
		redisContainer:    redisContainer,
		minioDep:          minioDep,
		mmrExtConfigPath:  tmpPath,
		mmrSigningKeyPath: signingKeyFile.Name(),
		Homeservers:       []*SynapseDep{syn1, syn2},
		Machines:          mmrs,
		depNet:            depNet,
	}, nil
}

func (c *ContainerDeps) Teardown() {
	for _, machine := range c.Machines {
		machine.Teardown()
	}
	for _, hs := range c.Homeservers {
		hs.Teardown()
	}
	if err := c.redisContainer.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down redis container: %s", err.Error())
	}
	if err := c.pgContainer.Terminate(c.ctx); err != nil {
		log.Fatalf("Error shutting down mmr-postgres container: %s", err.Error())
	}
	c.minioDep.Teardown()
	if err := os.Remove(c.mmrExtConfigPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error cleaning up MMR-External config file '%s': %s", c.mmrExtConfigPath, err.Error())
	}
	if err := os.Remove(c.mmrSigningKeyPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error cleaning up MMR-Signing Key file '%s': %s", c.mmrSigningKeyPath, err.Error())
	}

	// XXX: We should be shutting this down, but it appears testcontainers leaves something attached :(
	//c.depNet.Teardown()
}

func (c *ContainerDeps) Debug() {
	for i, m := range c.Machines {
		logs, err := m.Logs()
		c.DumpDebugLogs(logs, err, i, m.HttpUrl)
	}
}

func (c *ContainerDeps) DumpDebugLogs(logs io.ReadCloser, err error, i int, url string) {
	if err != nil {
		log.Fatal(err)
	}
	b, err := io.ReadAll(logs)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[MMR Deps] Logs from index %d (%s)", i, url)
	fmt.Println()
	fmt.Println(string(b))
	err = logs.Close()
	if err != nil {
		log.Fatal(err)
	}
}
