package test

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/test/test_internals"
	"github.com/t2bot/matrix-media-repo/util"
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
		AccessToken:     s.deps.Homeservers[1].UnprivilegedUsers[0].AccessToken,
		UserId:          s.deps.Homeservers[1].UnprivilegedUsers[0].UserId,
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

	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
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

func (s *UploadTestSuite) TestUploadSpam() {
	t := s.T()
	const concurrentUploads = 100

	// Clients are for the same user/server, but using different MMR machines
	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[1].HttpUrl)
	assert.Equal(t, client1.ServerName, client2.ServerName)

	// Create the image streams first (so we don't accidentally hit slowdowns during upload)
	images := make([]io.Reader, concurrentUploads)
	contentTypes := make([]string, concurrentUploads)
	for i := 0; i < concurrentUploads; i++ {
		c, img, err := test_internals.MakeTestImage(128, 128)
		assert.NoError(t, err)
		images[i] = img
		contentTypes[i] = c
	}

	// Start all the uploads concurrently, and wait for them to complete
	waiter := new(sync.WaitGroup)
	waiter.Add(1)
	uploadWaiter := new(sync.WaitGroup)
	mediaIds := new(sync.Map)
	for i := 0; i < concurrentUploads; i++ {
		uploadWaiter.Add(1)
		go func(j int) {
			defer uploadWaiter.Done()

			img := images[j]
			contentType := contentTypes[j]
			client := client1
			if rand.Float32() < 0.5 {
				client = client2
			}
			waiter.Wait()

			// We use random file names to guarantee different media IDs at a minimum
			rstr, err := util.GenerateRandomString(64)
			assert.NoError(t, err)
			res, err := client.Upload("image"+rstr+util.ExtensionForContentType(contentType), contentType, img)
			assert.NoError(t, err)
			assert.NotEmpty(t, res.MxcUri)

			origin, mediaId, err := util.SplitMxc(res.MxcUri)
			assert.NoError(t, err)
			assert.Equal(t, client.ServerName, origin)
			assert.NotEmpty(t, mediaId)
			mediaIds.Store(mediaId, true)
		}(i)
	}
	waiter.Done()
	uploadWaiter.Wait()

	// Prepare to check that only one copy of the file was uploaded each time
	mediaDb := database.GetInstance().Media.Prepare(rcontext.Initial())
	realMediaIds := make([]string, 0)
	mediaIds.Range(func(key any, value any) bool {
		realMediaIds = append(realMediaIds, key.(string))
		return true
	})
	assert.Greater(t, len(realMediaIds), 0)
	records, err := mediaDb.GetByIds(client1.ServerName, realMediaIds)
	assert.NoError(t, err)
	assert.NotNil(t, records)
	assert.Len(t, records, len(realMediaIds))

	// Actually do the comparison
	dsId := records[0].DatastoreId
	dsLocation := records[0].Location
	for _, r := range records {
		assert.Equal(t, dsId, r.DatastoreId)
		assert.Equal(t, dsLocation, r.Location)
	}
}

func (s *UploadTestSuite) TestUploadAsyncFlow() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)
	client2 := &test_internals.MatrixClient{
		ClientServerUrl: s.deps.Machines[1].HttpUrl,       // deliberately the second machine
		ServerName:      s.deps.Homeservers[1].ServerName, // deliberately the second machine
		AccessToken:     s.deps.Homeservers[1].UnprivilegedUsers[0].AccessToken,
		UserId:          s.deps.Homeservers[1].UnprivilegedUsers[0].UserId,
	}

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)

	response := new(test_internals.MatrixCreatedMediaResponse)
	err = client1.DoReturnJson("POST", "/_matrix/media/v1/create", nil, "", nil, response)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.MxcUri)
	assert.Greater(t, response.ExpiresTs, int64(0))

	origin, mediaId, err := util.SplitMxc(response.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin)
	assert.NotEmpty(t, mediaId)

	// Do a test download to ensure that the media doesn't (yet) exist
	errRes, err := client2.DoExpectError("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), url.Values{
		"timeout_ms": []string{"1000"},
	}, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, errRes)
	assert.Equal(t, "M_NOT_YET_UPLOADED", errRes.Code)
	assert.Equal(t, http.StatusGatewayTimeout, errRes.InjectedStatusCode)

	// Do the upload
	uploadResponse := new(test_internals.MatrixUploadResponse)
	err = client1.DoReturnJson("PUT", fmt.Sprintf("/_matrix/media/v3/upload/%s/%s", origin, mediaId), nil, contentType, img, uploadResponse)
	assert.NoError(t, err)
	assert.NotNil(t, uploadResponse)
	assert.Empty(t, uploadResponse.MxcUri) // not returned by this endpoint

	// Upload again, expecting error
	contentType, img, err = test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)
	errRes, err = client1.DoExpectError("PUT", fmt.Sprintf("/_matrix/media/v3/upload/%s/%s", origin, mediaId), nil, contentType, img)
	assert.NoError(t, err)
	assert.NotNil(t, errRes)
	assert.Equal(t, "M_CANNOT_OVERWRITE_MEDIA", errRes.Code)
	assert.Equal(t, http.StatusConflict, errRes.InjectedStatusCode)

	// Download and test the upload
	raw, err := client2.DoRaw("GET", fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", origin, mediaId), nil, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, raw.StatusCode, http.StatusOK)
	test_internals.AssertIsTestImage(t, raw.Body)
}

func (s *UploadTestSuite) TestUploadAsyncExpiredFlow() {
	t := s.T()

	client1 := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	assert.NoError(t, err)

	response := new(test_internals.MatrixCreatedMediaResponse)
	err = client1.DoReturnJson("POST", "/_matrix/media/v1/create", nil, "", nil, response)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.MxcUri)
	assert.Greater(t, response.ExpiresTs, int64(0))

	origin, mediaId, err := util.SplitMxc(response.MxcUri)
	assert.NoError(t, err)
	assert.Equal(t, client1.ServerName, origin)
	assert.NotEmpty(t, mediaId)

	// Expire the media
	db := database.GetAccessorForTests()
	_, err = db.Exec("UPDATE expiring_media SET expires_ts = 5 WHERE origin = $1 AND media_id = $2;", origin, mediaId)
	assert.NoError(t, err)

	// Upload, expecting error
	errRes, err := client1.DoExpectError("PUT", fmt.Sprintf("/_matrix/media/v3/upload/%s/%s", origin, mediaId), nil, contentType, img)
	assert.NoError(t, err)
	assert.NotNil(t, errRes)
	assert.Equal(t, "M_NOT_FOUND", errRes.Code)
	assert.Equal(t, http.StatusNotFound, errRes.InjectedStatusCode)
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}
