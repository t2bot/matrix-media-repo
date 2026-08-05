package test

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2bot/matrix-media-repo/test/test_internals"
	"github.com/testcontainers/testcontainers-go"
)

type MSC3916MiscClientEndpointsSuite struct {
	suite.Suite
	deps        *test_internals.ContainerDeps
	htmlPage    *httptest.Server
	htmlPageURL string
}

func (s *MSC3916MiscClientEndpointsSuite) SetupSuite() {
	htmlPage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<h1>This is a test file</h1>"))
	}))
	htmlPagePort := htmlPage.Listener.Addr().(*net.TCPAddr).Port

	deps, err := test_internals.MakeTestDeps(htmlPagePort)
	if err != nil {
		htmlPage.Close()
		log.Fatal(err)
	}
	s.deps = deps
	s.htmlPage = htmlPage
	s.htmlPageURL = fmt.Sprintf(
		"https://%s:%d/index.html",
		testcontainers.HostInternal,
		htmlPagePort,
	)
}

func (s *MSC3916MiscClientEndpointsSuite) TearDownSuite() {
	if s.deps != nil {
		if s.T().Failed() {
			s.deps.Debug()
		}
		s.deps.Teardown()
	}
	if s.htmlPage != nil {
		s.htmlPage.Close()
	}
}

func (s *MSC3916MiscClientEndpointsSuite) TestPreviewUrlRequiresAuth() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := &test_internals.MatrixClient{
		ClientServerUrl: s.deps.Machines[0].HttpUrl,
		ServerName:      s.deps.Homeservers[0].ServerName,
		AccessToken:     "", // no auth on this client
		UserId:          "", // no auth on this client
	}
	clientGuest := s.deps.Homeservers[0].GuestUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	qs := url.Values{
		"url": []string{s.htmlPageURL},
	}
	raw, err := client2.DoRaw("GET", "/_matrix/client/v1/media/preview_url", qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = clientGuest.DoRaw("GET", "/_matrix/client/v1/media/preview_url", qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, raw.StatusCode)

	raw, err = client1.DoRaw("GET", "/_matrix/client/v1/media/preview_url", qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func (s *MSC3916MiscClientEndpointsSuite) TestConfigRequiresAuth() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := &test_internals.MatrixClient{
		ClientServerUrl: s.deps.Machines[0].HttpUrl,
		ServerName:      s.deps.Homeservers[0].ServerName,
		AccessToken:     "", // no auth on this client
		UserId:          "", // no auth on this client
	}
	clientGuest := s.deps.Homeservers[0].GuestUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	raw, err := client2.DoRaw("GET", "/_matrix/client/v1/media/config", nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = clientGuest.DoRaw("GET", "/_matrix/client/v1/media/config", nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, raw.StatusCode)

	raw, err = client1.DoRaw("GET", "/_matrix/client/v1/media/config", nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func TestMSC3916MiscClientEndpointsSuite(t *testing.T) {
	suite.Run(t, new(MSC3916MiscClientEndpointsSuite))
}
