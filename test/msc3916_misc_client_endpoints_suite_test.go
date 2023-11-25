package test

import (
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/turt2live/matrix-media-repo/test/test_internals"
)

type MSC3916MiscClientEndpointsSuite struct {
	suite.Suite
	deps     *test_internals.ContainerDeps
	htmlPage *test_internals.HostedFile
}

func (s *MSC3916MiscClientEndpointsSuite) SetupSuite() {
	deps, err := test_internals.MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps

	file, err := test_internals.ServeFile("index.html", deps, "<h1>This is a test file</h1>")
	if err != nil {
		log.Fatal(err)
	}
	s.htmlPage = file
}

func (s *MSC3916MiscClientEndpointsSuite) TearDownSuite() {
	if s.htmlPage != nil {
		s.htmlPage.Teardown()
	}
	if s.deps != nil {
		if s.T().Failed() {
			s.deps.Debug()
		}
		s.deps.Teardown()
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

	qs := url.Values{
		"url": []string{s.htmlPage.PublicUrl},
	}
	raw, err := client2.DoRaw("GET", "/_matrix/client/unstable/org.matrix.msc3916/media/preview_url", qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = client1.DoRaw("GET", "/_matrix/client/unstable/org.matrix.msc3916/media/preview_url", qs, "", nil)
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

	raw, err := client2.DoRaw("GET", "/_matrix/client/unstable/org.matrix.msc3916/media/config", nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = client1.DoRaw("GET", "/_matrix/client/unstable/org.matrix.msc3916/media/config", nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func TestMSC3916MiscClientEndpointsSuite(t *testing.T) {
	suite.Run(t, new(MSC3916MiscClientEndpointsSuite))
}
