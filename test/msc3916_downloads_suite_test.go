package test

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/test/test_internals"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/testcontainers/testcontainers-go"
)

type MSC3916DownloadsSuite struct {
	suite.Suite
	deps         *test_internals.ContainerDeps
	keyServer    *test_internals.HostedFile
	keyServerKey *homeserver_interop.SigningKey
}

func (s *MSC3916DownloadsSuite) SetupSuite() {
	err := os.Setenv("MEDIA_REPO_HTTP_ONLY_FEDERATION", "true")
	if err != nil {
		s.T().Fatal(err)
	}

	deps, err := test_internals.MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps

	s.keyServer, s.keyServerKey = test_internals.MakeKeyServer(deps)
}

func (s *MSC3916DownloadsSuite) TearDownSuite() {
	err := os.Unsetenv("MEDIA_REPO_HTTP_ONLY_FEDERATION")
	if err != nil {
		s.T().Fatal(err)
	}
	if s.deps != nil {
		if s.T().Failed() {
			s.deps.Debug()
		}
		s.deps.Teardown()
	}
}

func (s *MSC3916DownloadsSuite) TestClientDownloads() {
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

	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)
	raw, err = client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s/whatever.png", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	test_internals.AssertIsTestImage(t, raw.Body)
	raw, err = client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s/whatever.png", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	test_internals.AssertIsTestImage(t, raw.Body)

	raw, err = clientGuest.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	test_internals.AssertIsTestImage(t, raw.Body)
	raw, err = clientGuest.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s/whatever.png", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	test_internals.AssertIsTestImage(t, raw.Body)
}

func (s *MSC3916DownloadsSuite) TestFederationDownloads() {
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
	uri := fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId)
	raw, err := remoteClient.DoRaw("GET", uri, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	// Now add the X-Matrix auth and try again
	header, err := matrix.CreateXMatrixHeader(s.keyServer.PublicHostname, remoteClient.ServerName, "GET", uri, nil, s.keyServerKey.PrivateKey, s.keyServerKey.KeyVersion)
	assert.NoError(t, err)
	remoteClient.AuthHeaderOverride = header
	raw, err = remoteClient.DoRaw("GET", uri, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func (s *MSC3916DownloadsSuite) TestFederationMakesAuthedDownloads() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	origin := ""
	mediaId := "abc123"
	err := matrix.TestsOnlyInjectSigningKey(s.deps.Homeservers[0].ServerName, s.deps.Homeservers[0].ExternalClientServerApiUrl)
	assert.NoError(t, err)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId), r.URL.Path)
		origin, err := matrix.ValidateXMatrixAuth(r, true)
		assert.NoError(t, err)
		assert.Equal(t, client1.ServerName, origin)
		w.Header().Set("Content-Type", "multipart/mixed; boundary=gc0p4Jq0M2Yt08jU534c0p")
		_, _ = w.Write([]byte("--gc0p4Jq0M2Yt08jU534c0p\nContent-Type: application/json\n\n{}\n\n--gc0p4Jq0M2Yt08jU534c0p\nContent-Type: text/plain\n\nThis media is plain text. Maybe somebody used it as a paste bin.\n\n--gc0p4Jq0M2Yt08jU534c0p"))
	}))
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	origin = fmt.Sprintf("%s:%s", testcontainers.HostInternal, u.Port())
	config.AddDomainForTesting(testcontainers.HostInternal, nil) // no port for config lookup

	raw, err := client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func (s *MSC3916DownloadsSuite) TestFederationFollowsRedirects() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	origin := ""
	mediaId := "abc123"
	fileContents := "hello world! This is a test file"
	err := matrix.TestsOnlyInjectSigningKey(s.deps.Homeservers[0].ServerName, s.deps.Homeservers[0].ExternalClientServerApiUrl)
	assert.NoError(t, err)

	// Mock CDN (2nd hop)
	testServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/cdn/file", r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fileContents))
	}))
	defer testServer2.Close()
	u, _ := url.Parse(testServer2.URL)
	//goland:noinspection HttpUrlsUsage
	redirectUrl := fmt.Sprintf("http://%s:%s/cdn/file", testcontainers.HostInternal, u.Port())

	// Mock homeserver (1st hop)
	testServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId), r.URL.Path)
		origin, err := matrix.ValidateXMatrixAuth(r, true)
		assert.NoError(t, err)
		assert.Equal(t, client1.ServerName, origin)
		w.Header().Set("Content-Type", "multipart/mixed; boundary=gc0p4Jq0M2Yt08jU534c0p")
		_, _ = w.Write([]byte(fmt.Sprintf("--gc0p4Jq0M2Yt08jU534c0p\nContent-Type: application/json\n\n{}\n\n--gc0p4Jq0M2Yt08jU534c0p\nLocation: %s\n\n-gc0p4Jq0M2Yt08jU534c0p", redirectUrl)))
	}))
	defer testServer1.Close()

	u, _ = url.Parse(testServer1.URL)
	origin = fmt.Sprintf("%s:%s", testcontainers.HostInternal, u.Port())
	config.AddDomainForTesting(testcontainers.HostInternal, nil) // no port for config lookup

	raw, err := client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)

	b, err := io.ReadAll(raw.Body)
	assert.NoError(t, err)
	assert.Equal(t, fileContents, string(b))
}

func (s *MSC3916DownloadsSuite) TestFederationProducesRedirects() {
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
	uri := fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId)
	header, err := matrix.CreateXMatrixHeader(s.keyServer.PublicHostname, remoteClient.ServerName, "GET", uri, nil, s.keyServerKey.PrivateKey, s.keyServerKey.KeyVersion)
	assert.NoError(t, err)
	remoteClient.AuthHeaderOverride = header
	raw, err := remoteClient.DoRaw("GET", uri, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)

	// TODO: Need to actually test that redirects are properly formed, and set up the test suite to produce them
}

func (s *MSC3916DownloadsSuite) TestFederationMakesAuthedDownloadsAndFallsBack() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	origin := ""
	mediaId := "abc123"
	fileContents := "hello world! This is a test file"
	err := matrix.TestsOnlyInjectSigningKey(s.deps.Homeservers[0].ServerName, s.deps.Homeservers[0].ExternalClientServerApiUrl)
	assert.NoError(t, err)

	reqNum := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqNum == 0 {
			origin, err := matrix.ValidateXMatrixAuth(r, true)
			assert.NoError(t, err)
			assert.Equal(t, client1.ServerName, origin)
			assert.Equal(t, fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId), r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("{\"errcode\":\"M_UNRECOGNIZED\"}"))
			reqNum++
		} else {
			assert.Equal(t, fmt.Sprintf("/_matrix/media/v3/download/%s/%s", origin, mediaId), r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fileContents))
	}))
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	origin = fmt.Sprintf("%s:%s", testcontainers.HostInternal, u.Port())
	config.AddDomainForTesting(testcontainers.HostInternal, nil) // no port for config lookup

	raw, err := client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)

	b, err := io.ReadAll(raw.Body)
	assert.NoError(t, err)
	assert.Equal(t, fileContents, string(b))
}

func TestMSC3916DownloadsSuite(t *testing.T) {
	suite.Run(t, new(MSC3916DownloadsSuite))
}
