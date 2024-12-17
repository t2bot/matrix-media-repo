package test

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/test/test_internals"
	"github.com/t2bot/matrix-media-repo/util"
)

type MSC3916ThumbnailsSuite struct {
	suite.Suite
	deps         *test_internals.ContainerDeps
	keyServer    *test_internals.HostedFile
	keyServerKey *homeserver_interop.SigningKey
}

func (s *MSC3916ThumbnailsSuite) SetupSuite() {
	deps, err := test_internals.MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps

	s.keyServer, s.keyServerKey = test_internals.MakeKeyServer(deps)
}

func (s *MSC3916ThumbnailsSuite) TearDownSuite() {
	if s.deps != nil {
		if s.T().Failed() {
			s.deps.Debug()
		}
		s.deps.Teardown()
	}
}

func (s *MSC3916ThumbnailsSuite) TestClientThumbnails() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := &test_internals.MatrixClient{
		ClientServerUrl: s.deps.Machines[0].HttpUrl,
		ServerName:      s.deps.Homeservers[0].ServerName,
		AccessToken:     "", // this client isn't authed
		UserId:          "", // this client isn't authed
	}
	clientGuest := s.deps.Homeservers[0].GuestUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	fname := "image" + util.ExtensionForContentType(contentType)

	res, err := client1.Upload(fname, contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res.MxcUri)

	origin, mediaId, err := util.SplitMxc(res.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin)
	assert.NotEmpty(t, mediaId)

	qs := url.Values{
		"width":  []string{"96"},
		"height": []string{"96"},
		"method": []string{"scale"},
	}

	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/thumbnail/%s/%s", origin, mediaId), qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/thumbnail/%s/%s", origin, mediaId), qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	//test_internals.AssertIsTestImage(t, raw.Body) // we can't verify that the resulting image is correct

	raw, err = clientGuest.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/thumbnail/%s/%s", origin, mediaId), qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	//test_internals.AssertIsTestImage(t, raw.Body) // we can't verify that the resulting image is correct
}

func (s *MSC3916ThumbnailsSuite) TestFederationThumbnails() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	remoteClient := &test_internals.MatrixClient{
		ClientServerUrl: s.deps.Machines[0].HttpUrl,
		ServerName:      s.deps.Homeservers[0].ServerName,
		AccessToken:     "", // this client isn't authed over the CS API
		UserId:          "", // this client isn't authed over the CS API
	}

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	fname := "image" + util.ExtensionForContentType(contentType)

	res, err := client1.Upload(fname, contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res.MxcUri)

	origin, mediaId, err := util.SplitMxc(res.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin)
	assert.NotEmpty(t, mediaId)

	// Verify the federation download *fails* when lacking auth
	uri := fmt.Sprintf("/_matrix/federation/v1/media/thumbnail/%s", mediaId)
	qs := url.Values{
		"width":  []string{"96"},
		"height": []string{"96"},
		"method": []string{"scale"},
	}
	raw, err := remoteClient.DoRaw("GET", uri, qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	// Now add the X-Matrix auth and try again
	header, err := matrix.CreateXMatrixHeader(s.keyServer.PublicHostname, remoteClient.ServerName, "GET", fmt.Sprintf("%s?%s", uri, qs.Encode()), nil, s.keyServerKey.PrivateKey, s.keyServerKey.KeyVersion)
	assert.NoError(t, err)
	remoteClient.AuthHeaderOverride = header
	raw, err = remoteClient.DoRaw("GET", uri, qs, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func TestMSC3916ThumbnailsSuite(t *testing.T) {
	suite.Run(t, new(MSC3916ThumbnailsSuite))
}
