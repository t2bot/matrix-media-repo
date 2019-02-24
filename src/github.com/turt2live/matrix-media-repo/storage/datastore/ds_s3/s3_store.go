package ds_s3

import (
	"context"
	"io"
	"strconv"

	"github.com/minio/minio-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/util"
)

var stores = make(map[string]*s3Datastore)

type s3Datastore struct {
	conf   config.DatastoreConfig
	dsId   string
	client *minio.Client
	bucket string
}

func GetOrCreateS3Datastore(dsId string, conf config.DatastoreConfig) (*s3Datastore, error) {
	if s, ok := stores[dsId]; ok {
		return s, nil
	}

	endpoint, epFound := conf.Options["endpoint"]
	bucket, bucketFound := conf.Options["bucketName"]
	accessKeyId, keyFound := conf.Options["accessKeyId"]
	accessSecret, secretFound := conf.Options["accessSecret"]
	if !epFound || !bucketFound || !keyFound || !secretFound {
		return nil, errors.New("invalid configuration: missing s3 options")
	}

	useSsl := true
	useSslStr, sslFound := conf.Options["ssl"]
	if sslFound && useSslStr != "" {
		useSsl, _ = strconv.ParseBool(useSslStr)
	}

	s3client, err := minio.New(endpoint, accessKeyId, accessSecret, useSsl)
	if err != nil {
		return nil, err
	}

	s3ds := &s3Datastore{
		conf:   conf,
		dsId:   dsId,
		client: s3client,
		bucket: bucket,
	}
	stores[dsId] = s3ds
	return s3ds, nil
}

func (s *s3Datastore) UploadFile(file io.Reader, ctx context.Context, log *logrus.Entry) (string, error) {
	objectName, err := util.GenerateRandomString(128)
	if err != nil {
		return "", err
	}
	sizeBytes, err := s.client.PutObjectWithContext(ctx, s.bucket, objectName, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	log.Info("Uploaded ", sizeBytes, " bytes to s3")
	return objectName, nil
}
