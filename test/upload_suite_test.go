package test

import (
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
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
	assert.NoError(t, err)
	res, err := client1.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res.MxcUri)

	origin, mediaId, err := util.SplitMxc(res.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin)
	assert.NotEmpty(t, mediaId)

	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/media/v3/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, raw.StatusCode, http.StatusOK)
	test_internals.AssertIsTestImage(t, raw.Body)
}

func (s *UploadTestSuite) TestUploadDeduplicationSameUser() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	res1, err := client1.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res1.MxcUri)

	origin, mediaId, err := util.SplitMxc(res1.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin)
	assert.NotEmpty(t, mediaId)

	contentType, img, err = test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	res2, err := client1.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res2.MxcUri)

	assert.Equal(t, res1.MxcUri, res2.MxcUri)
}

func (s *UploadTestSuite) TestUploadDeduplicationSameUserDifferentMetadata() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	res1, err := client1.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res1.MxcUri)

	origin1, mediaId1, err := util.SplitMxc(res1.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin1)
	assert.NotEmpty(t, mediaId1)

	contentType, img, err = test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	res2, err := client1.Upload("DIFFERENT_FILE_NAME_SHOULD_GIVE_DIFFERENT_MEDIA_ID"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res2.MxcUri)

	origin2, mediaId2, err := util.SplitMxc(res2.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin2) // still the same server though
	assert.NotEmpty(t, mediaId2)

	assert.NotEqual(t, res1.MxcUri, res2.MxcUri) // should be different media IDs

	// Inspect database to ensure file was reused rather than uploaded twice
	mediaDb := database.GetInstance().Media.Prepare(rcontext.Initial())
	records, err := mediaDb.GetByIds(origin1, []string{mediaId1, mediaId2})
	assert.NoError(t, err)
	assert.NotNil(t, records)
	assert.Len(t, records, 2)
	assert.NotEqual(t, records[0].MediaId, records[1].MediaId)
	assert.Equal(t, records[0].DatastoreId, records[1].DatastoreId)
	assert.Equal(t, records[0].Location, records[1].Location)
}

func (s *UploadTestSuite) TestUploadDeduplicationDifferentUser() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := s.deps.Homeservers[1].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[1].HttpUrl)

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	res1, err := client1.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res1.MxcUri)

	origin1, mediaId1, err := util.SplitMxc(res1.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin1)
	assert.NotEmpty(t, mediaId1)

	contentType, img, err = test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	res2, err := client2.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	assert.NotEmpty(t, res2.MxcUri)

	origin2, mediaId2, err := util.SplitMxc(res2.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client2.ServerName, origin2)
	assert.NotEmpty(t, mediaId2)

	assert.NotEqual(t, res1.MxcUri, res2.MxcUri) // should be different URIs

	// Inspect database to ensure file was reused rather than uploaded twice
	mediaDb := database.GetInstance().Media.Prepare(rcontext.Initial())
	record1, err := mediaDb.GetById(origin1, mediaId1)
	assert.NoError(t, err)
	assert.NotNil(t, record1)
	record2, err := mediaDb.GetById(origin2, mediaId2)
	assert.NoError(t, err)
	assert.NotNil(t, record1)
	assert.Equal(t, record1.DatastoreId, record2.DatastoreId)
	assert.Equal(t, record1.Location, record2.Location)
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}
