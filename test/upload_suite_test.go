package test

import (
	"log"
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

	client := s.deps.Homeservers[0].UnprivilegedUsers[0].WithCsUrl(s.deps.Machines[0].HttpUrl)

	contentType, img, err := test_internals.MakeTestImage(512, 512)
	res, err := client.Upload("image"+util.ExtensionForContentType(contentType), contentType, img)
	assert.NoError(t, err)
	log.Println(res.MxcUri)
	assert.NotEmpty(t, res.MxcUri)
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}
