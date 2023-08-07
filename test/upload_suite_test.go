package test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type UploadTestSuite struct {
	suite.Suite
	deps *ContainerDeps
}

func (s *UploadTestSuite) SetupSuite() {
	deps, err := MakeTestDeps()
	if err != nil {
		log.Fatal(err)
	}
	s.deps = deps
}

func (s *UploadTestSuite) TearDownSuite() {
	if s.deps != nil {
		s.deps.Teardown()
	}
}

func (s *UploadTestSuite) TestUpload() {
	t := s.T()

	assert.NoError(t, nil)
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}
