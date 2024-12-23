package test_internals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

	InternalClientServerApiUrl string
	ExternalClientServerApiUrl string
	ServerName                 string

	AdminUsers        []*MatrixClient // uses ExternalClientServerApiUrl
	UnprivilegedUsers []*MatrixClient // uses ExternalClientServerApiUrl
	GuestUsers        []*MatrixClient // uses ExternalClientServerApiUrl
}

func MakeSynapse(domainName string, depNet *NetworkDep, signingKeyFilePath string) (*SynapseDep, error) {
	ctx := context.Background()

	// Start postgresql database
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/library/postgres:16"),
		postgres.WithDatabase("synapse"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test1234"),
		WithEnvironment("POSTGRES_INITDB_ARGS", "--encoding=UTF8 --locale=C"),
		depNet.ApplyToContainer(),
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
	w := new(strings.Builder)
	err = t.Execute(w, synapseTmplArgs{
		ServerName: domainName,
		PgHost:     pghost,
		PgPort:     5432, // we're behind the network here
	})
	if err != nil {
		return nil, err
	}

	// Write the synapse config to a temp file
	f, err := os.CreateTemp(os.TempDir(), "mmr-tests-synapse")
	if err != nil {
		return nil, err
	}
	err = f.Chmod(0644)
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
	err = os.Chmod(d, 0777)
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	synContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "ghcr.io/element-hq/synapse:v1.121.1",
			ExposedPorts: []string{"8008/tcp"},
			Mounts: []testcontainers.ContainerMount{
				testcontainers.BindMount(f.Name(), "/data/homeserver.yaml"),
				testcontainers.BindMount(signingKeyFilePath, "/data/signing.key"),
				testcontainers.BindMount(path.Join(cwd, ".", "test", "templates", "synapse.log.config"), "/data/log.config"),
				testcontainers.BindMount(d, "/app"),
			},
			WaitingFor: wait.ForHTTP("/health").WithPort(p),
			Networks:   []string{depNet.NetId},
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
	synIp, err := synContainer.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}
	synPort, err := synContainer.MappedPort(ctx, "8008/tcp")
	if err != nil {
		return nil, err
	}
	//goland:noinspection HttpUrlsUsage
	intCsApiUrl := fmt.Sprintf("http://%s:%d", synIp, 8008)
	extCsApiUrl := fmt.Sprintf("http://%s:%d", synHost, synPort.Int())

	// Register the accounts
	adminUsers := make([]*MatrixClient, 0)
	unprivilegedUsers := make([]*MatrixClient, 0)
	guestUsers := make([]*MatrixClient, 0)
	registerUser := func(localpart string, admin bool) error {
		adminFlag := "--admin"
		if !admin {
			adminFlag = "--no-admin"
		}
		cmd := fmt.Sprintf("register_new_matrix_user -c /data/homeserver.yaml -u %s -p test1234 %s", localpart, adminFlag)
		log.Println("[Synapse Command] " + cmd)
		i, r, err := synContainer.Exec(ctx, strings.Split(cmd, " "))
		if err != nil {
			return err
		}
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if i != 0 {
			return errors.New(string(b))
		}

		// Get user ID and access token from login API
		log.Println("[Synapse API] Logging in")
		endpoint, err := url.JoinPath(extCsApiUrl, "/_matrix/client/v3/login")
		if err != nil {
			return err
		}
		b, err = json.Marshal(map[string]interface{}{
			"type": "m.login.password",
			"identifier": map[string]interface{}{
				"type": "m.id.user",
				"user": localpart,
			},
			"password":      "test1234",
			"refresh_token": false,
		})
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Post(endpoint, "application/json", bytes.NewBuffer(b))
		if err != nil {
			return err
		}
		b, err = io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			return errors.New(res.Status + "\n" + string(b))
		}
		log.Println("[Synapse API] " + string(b))
		m := make(map[string]interface{})
		err = json.Unmarshal(b, &m)
		if err != nil {
			return err
		}

		var userId interface{}
		var accessToken interface{}
		var ok bool
		if userId, ok = m["user_id"]; !ok {
			return errors.New("missing user_id")
		}
		if accessToken, ok = m["access_token"]; !ok {
			return errors.New("missing access_token")
		}

		mxClient := &MatrixClient{
			AccessToken:     accessToken.(string),
			ClientServerUrl: extCsApiUrl,
			UserId:          userId.(string),
			ServerName:      domainName,
		}

		if admin {
			adminUsers = append(adminUsers, mxClient)
		} else {
			unprivilegedUsers = append(unprivilegedUsers, mxClient)
		}

		return nil
	}
	registerGuest := func(localpart string) error {
		log.Println("[Synapse API] Register guest")
		u, err := url.Parse(extCsApiUrl + "/_matrix/client/v3/register?kind=guest")
		if err != nil {
			return err
		}
		b, err := json.Marshal(map[string]interface{}{})
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Post(u.String(), "application/json", bytes.NewBuffer(b))
		if err != nil {
			return err
		}
		b, err = io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			return errors.New(res.Status + "\n" + string(b))
		}
		log.Println("[Synapse API] " + string(b))
		m := make(map[string]interface{})
		err = json.Unmarshal(b, &m)
		if err != nil {
			return err
		}

		var userId interface{}
		var accessToken interface{}
		var ok bool
		if userId, ok = m["user_id"]; !ok {
			return errors.New("missing user_id")
		}
		if accessToken, ok = m["access_token"]; !ok {
			return errors.New("missing access_token")
		}

		mxClient := &MatrixClient{
			AccessToken:     accessToken.(string),
			ClientServerUrl: extCsApiUrl,
			UserId:          userId.(string),
			ServerName:      domainName,
		}

		guestUsers = append(guestUsers, mxClient)

		return nil
	}
	err = registerUser("admin", true)
	if err != nil {
		return nil, err
	}
	err = registerUser("user_alice", false)
	if err != nil {
		return nil, err
	}
	err = registerUser("user_bob", false)
	if err != nil {
		return nil, err
	}
	err = registerGuest("user_guest")
	if err != nil {
		return nil, err
	}

	// Create the dependency
	return &SynapseDep{
		ctx:                        ctx,
		pgContainer:                pgContainer,
		synContainer:               synContainer,
		tmpConfigPath:              f.Name(),
		InternalClientServerApiUrl: intCsApiUrl,
		ExternalClientServerApiUrl: extCsApiUrl,
		ServerName:                 domainName,
		AdminUsers:                 adminUsers,
		UnprivilegedUsers:          unprivilegedUsers,
		GuestUsers:                 guestUsers,
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
