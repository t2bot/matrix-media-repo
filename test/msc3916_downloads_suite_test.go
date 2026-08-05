package test

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
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

type replaceableHandler struct {
	mutex   sync.RWMutex
	handler http.Handler
}

func (h *replaceableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mutex.RLock()
	handler := h.handler
	h.mutex.RUnlock()
	if handler == nil {
		http.NotFound(w, r)
		return
	}
	handler.ServeHTTP(w, r)
}

func (h *replaceableHandler) Set(handler http.Handler) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.handler = handler
}

func validateXMatrixAuthForDestination(request *http.Request, destination string) (string, error) {
	requestCopy := request.Clone(request.Context())
	requestCopy.Host = destination
	return matrix.ValidateXMatrixAuth(requestCopy, true)
}

type MSC3916DownloadsSuite struct {
	suite.Suite
	deps         *test_internals.ContainerDeps
	keyServer    *test_internals.HostedFile
	keyServerKey *homeserver_interop.SigningKey
	hostServer   *httptest.Server
	hostHandler  *replaceableHandler
}

func (s *MSC3916DownloadsSuite) SetupSuite() {
	err := os.Setenv("MEDIA_REPO_HTTP_ONLY_FEDERATION", "true")
	if err != nil {
		s.T().Fatal(err)
	}

	hostHandler := &replaceableHandler{}
	hostServer := httptest.NewServer(hostHandler)
	hostPort := hostServer.Listener.Addr().(*net.TCPAddr).Port

	deps, err := test_internals.MakeTestDeps(hostPort)
	if err != nil {
		hostServer.Close()
		log.Fatal(err)
	}
	s.deps = deps
	s.hostHandler = hostHandler
	s.hostServer = hostServer

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
	if s.hostServer != nil {
		s.hostServer.Close()
	}
}

func (s *MSC3916DownloadsSuite) hostOrigin() string {
	hostPort := s.hostServer.Listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("%s:%d", testcontainers.HostInternal, hostPort)
}

func (s *MSC3916DownloadsSuite) setHostHandler(handler http.Handler) {
	s.hostHandler.Set(handler)
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

	legacyDownloadPath := fmt.Sprintf("/_matrix/media/v3/download/%s/%s", origin, mediaId)
	raw, err := client2.DoRaw("GET", legacyDownloadPath, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, raw.StatusCode)

	raw, err = client1.DoRaw("GET", legacyDownloadPath, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	test_internals.AssertIsTestImage(t, raw.Body)

	raw, err = client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
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

	origin := s.hostOrigin()
	mediaId := "authed-download"
	err := matrix.TestsOnlyInjectSigningKey(s.deps.Homeservers[0].ServerName, s.deps.Homeservers[0].ExternalClientServerApiUrl)
	assert.NoError(t, err)
	s.setHostHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId), r.URL.Path)
		requestOrigin, validationErr := validateXMatrixAuthForDestination(r, origin)
		assert.NoError(t, validationErr)
		assert.Equal(t, client1.ServerName, requestOrigin)
		w.Header().Set("Content-Type", "multipart/mixed; boundary=gc0p4Jq0M2Yt08jU534c0p")
		_, _ = w.Write([]byte("--gc0p4Jq0M2Yt08jU534c0p\nContent-Type: application/json\n\n{}\n\n--gc0p4Jq0M2Yt08jU534c0p\nContent-Type: text/plain\n\nThis media is plain text. Maybe somebody used it as a paste bin.\n\n--gc0p4Jq0M2Yt08jU534c0p"))
	}))
	defer s.setHostHandler(nil)
	config.AddDomainForTesting(origin, nil)

	raw, err := client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

func (s *MSC3916DownloadsSuite) TestFederationFollowsRedirects() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	origin := s.hostOrigin()
	mediaId := "redirect-download"
	fileContents := "hello world! This is a test file"
	err := matrix.TestsOnlyInjectSigningKey(s.deps.Homeservers[0].ServerName, s.deps.Homeservers[0].ExternalClientServerApiUrl)
	assert.NoError(t, err)

	//goland:noinspection HttpUrlsUsage
	redirectUrl := fmt.Sprintf("http://%s/cdn/file", origin)
	s.setHostHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cdn/file":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(fileContents))
		case fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId):
			requestOrigin, validationErr := validateXMatrixAuthForDestination(r, origin)
			assert.NoError(t, validationErr)
			assert.Equal(t, client1.ServerName, requestOrigin)
			w.Header().Set("Content-Type", "multipart/mixed; boundary=gc0p4Jq0M2Yt08jU534c0p")
			_, _ = w.Write([]byte(fmt.Sprintf("--gc0p4Jq0M2Yt08jU534c0p\nContent-Type: application/json\n\n{}\n\n--gc0p4Jq0M2Yt08jU534c0p\nLocation: %s\n\n-gc0p4Jq0M2Yt08jU534c0p", redirectUrl)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.setHostHandler(nil)
	config.AddDomainForTesting(origin, nil)

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

	origin := s.hostOrigin()
	mediaId := "fallback-download"
	fileContents := "hello world! This is a test file"
	err := matrix.TestsOnlyInjectSigningKey(s.deps.Homeservers[0].ServerName, s.deps.Homeservers[0].ExternalClientServerApiUrl)
	assert.NoError(t, err)

	reqNum := 0
	s.setHostHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqNum == 0 {
			requestOrigin, validationErr := validateXMatrixAuthForDestination(r, origin)
			assert.NoError(t, validationErr)
			assert.Equal(t, client1.ServerName, requestOrigin)
			assert.Equal(t, fmt.Sprintf("/_matrix/federation/v1/media/download/%s", mediaId), r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("{\"errcode\":\"M_UNRECOGNIZED\"}"))
			reqNum++
			return
		}
		assert.Equal(t, fmt.Sprintf("/_matrix/media/v3/download/%s/%s", origin, mediaId), r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fileContents))
	}))
	defer s.setHostHandler(nil)
	config.AddDomainForTesting(origin, nil)

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
