package test

import (
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/turt2live/matrix-media-repo/test/test_internals"
	"github.com/turt2live/matrix-media-repo/util"
)

type MSC3916DownloadsSuite struct {
	suite.Suite
	deps *test_internals.ContainerDeps
}

func (s *MSC3916DownloadsSuite) SetupSuite() {
	deps, err := test_internals.MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps
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

	//res := new(test_internals.MatrixUploadResponse)
	//err = client1.DoReturnJson("POST", "/_matrix/client/unstable/org.matrix.msc3916/media/upload", url.Values{"filename": []string{fname}}, contentType, img, res)
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
	t.Error("Not yet implemented")
}

func TestMSC3916DownloadsSuite(t *testing.T) {
	suite.Run(t, new(MSC3916DownloadsSuite))
}
