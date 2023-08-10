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

type UploadTestSuite struct {
	suite.Suite
	deps *test_internals.ContainerDeps
}

func (s *UploadTestSuite) SetupSuite() {
	deps, err := test_internals.MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps
}

func (s *UploadTestSuite) TearDownSuite() {
	if s.deps != nil {
		if s.T().Failed() {
			s.deps.Debug()
		}
		s.deps.Teardown()
	}
}

func (s *UploadTestSuite) TestUpload() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := &test_internals.MatrixClient{
		ClientServerUrl: s.deps.Machines[1].HttpUrl,       // deliberately the second machine
		ServerName:      s.deps.Homeservers[1].ServerName, // deliberately the second machine
		AccessToken:     "",                               // no auth for downloads
		UserId:          "",                               // no auth for downloads
	}

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	res, err := client1.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res.MxcUri)

	origin, mediaId, err := util.SplitMxc(res.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, origin, client1.ServerName)
	assert.NotEmpty(t, mediaId)

	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/media/v3/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, raw.StatusCode, http.StatusOK)
	test_internals.AssertIsTestImage(t, raw.Body)
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}
