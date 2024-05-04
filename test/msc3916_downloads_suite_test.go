package test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/mmr"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/test/test_internals"
	"github.com/t2bot/matrix-media-repo/util"
)

type MSC3916DownloadsSuite struct {
	suite.Suite
	deps         *test_internals.ContainerDeps
	keyServer    *test_internals.HostedFile
	keyServerKey *homeserver_interop.SigningKey
}

func (s *MSC3916DownloadsSuite) SetupSuite() {
	deps, err := test_internals.MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps

	// We'll use a pre-computed signing key for simplicity
	signingKey, err := mmr.DecodeSigningKey(bytes.NewReader([]byte(`-----BEGIN MMR PRIVATE KEY-----
Key-ID: ed25519:e5d0oC
Version: 1

PJt0OaIImDJk8P/PDb4TNQHgI/1AA1C+AaQaABxAcgc=
-----END MMR PRIVATE KEY-----
`)))
	if err != nil {
		log.Fatal(err)
	}
	s.keyServerKey = signingKey
	// Create a /_matrix/key/v2/server response file (signed JSON)
	keyServer, writeFn, err := test_internals.LazyServeFile("_matrix/key/v2/server", deps)
	if err != nil {
		log.Fatal(err)
	}
	s.keyServer = keyServer
	serverKey := database.AnonymousJson{
		"old_verify_keys": database.AnonymousJson{},
		"server_name":     keyServer.PublicHostname,
		"valid_until_ts":  util.NowMillis() + (60 * 60 * 1000), // +1hr
		"verify_keys": database.AnonymousJson{
			"ed25519:e5d0oC": database.AnonymousJson{
				"key": "TohekYXzLx7VzV8FtLQlI3XsSdPv1CjhVYY5rZmFCvU",
			},
		},
	}
	canonical, err := util.EncodeCanonicalJson(serverKey)
	signature := util.EncodeUnpaddedBase64ToString(ed25519.Sign(signingKey.PrivateKey, canonical))
	serverKey["signatures"] = database.AnonymousJson{
		keyServer.PublicHostname: database.AnonymousJson{
			"ed25519:e5d0oC": signature,
		},
	}
	b, err := json.Marshal(serverKey)
	if err != nil {
		log.Fatal(err)
	}
	err = writeFn(string(b))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *MSC3916DownloadsSuite) TearDownSuite() {
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

	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/unstable/org.matrix.msc3916/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)
	raw, err = client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/unstable/org.matrix.msc3916/media/download/%s/%s/whatever.png", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	raw, err = client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/unstable/org.matrix.msc3916/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
	test_internals.AssertIsTestImage(t, raw.Body)
	raw, err = client1.DoRaw("GET", fmt.Sprintf("/_matrix/client/unstable/org.matrix.msc3916/media/download/%s/%s/whatever.png", origin, mediaId), nil, "", nil)
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
	uri := fmt.Sprintf("/_matrix/federation/unstable/org.matrix.msc3916/media/download/%s/%s", origin, mediaId)
	raw, err := remoteClient.DoRaw("GET", uri, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, raw.StatusCode)

	// Now add the X-Matrix auth and try again
	// TODO: We probably need to tell MMR to use an insecure environment to pass the federation test.
	header, err := matrix.CreateXMatrixHeader(s.keyServer.PublicHostname, remoteClient.ServerName, "GET", uri, &database.AnonymousJson{}, s.keyServerKey.PrivateKey, s.keyServerKey.KeyVersion)
	assert.NoError(t, err)
	remoteClient.AuthHeaderOverride = header
	raw, err = remoteClient.DoRaw("GET", uri, nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, raw.StatusCode)
}

//func (s *MSC3916DownloadsSuite) TestFederationMakesAuthedDownloads() {
//	t := s.T()
//
//	// TODO: Tests for:
//	// * Actually tries MSC3916 for downloads
//	// 		* Falls back on failure
//	// 		* Doesn't call unauthenticated endpoint if MSC3916 was successful
//	//		* Sets correct auth
//	t.Error("not yet implemented")
//}

func TestMSC3916DownloadsSuite(t *testing.T) {
	suite.Run(t, new(MSC3916DownloadsSuite))
}
