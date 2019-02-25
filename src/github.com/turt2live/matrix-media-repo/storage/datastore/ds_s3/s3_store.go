package ds_s3

import (
	"context"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/minio/minio-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/types"
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

func (s *s3Datastore) EnsureBucketExists() error {
	found, err := s.client.BucketExists(s.bucket)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("bucket not found")
	}
	return nil
}

func (s *s3Datastore) UploadFile(file io.ReadCloser, ctx context.Context, log *logrus.Entry) (*types.ObjectInfo, error) {
	objectName, err := util.GenerateRandomString(512)
	if err != nil {
		return nil, err
	}

	rs3, ws3 := io.Pipe()
	tr := io.TeeReader(file, ws3)

	done := make(chan bool)
	defer close(done)

	var hash string
	var sizeBytes int64
	var hashErr error
	var uploadErr error

	go func() {
		defer ws3.Close()
		log.Info("Calculating hash of stream...")
		hash, hashErr = util.GetSha256HashOfStream(ioutil.NopCloser(tr))
		log.Info("Hash of file is ", hash)
		done <- true
	}()

	go func() {
		log.Info("Uploading file...")
		sizeBytes, uploadErr = s.client.PutObjectWithContext(ctx, s.bucket, objectName, rs3, -1, minio.PutObjectOptions{})
		log.Info("Uploaded ", sizeBytes, " bytes to s3")
		done <- true
	}()

	for c := 0; c < 2; c++ {
		<-done
	}

	obj := &types.ObjectInfo{
		Location:   objectName,
		Sha256Hash: hash,
		SizeBytes:  sizeBytes,
	}

	if hashErr != nil {
		s.DeleteObject(obj.Location)
		return nil, hashErr
	}

	if uploadErr != nil {
		return nil, uploadErr
	}

	return obj, nil
}

func (s *s3Datastore) DeleteObject(location string) error {
	logrus.Info("Deleting object from bucket ", s.bucket, ": ", location)
	return s.client.RemoveObject(s.bucket, location)
}

func (s *s3Datastore) DownloadObject(location string) (io.ReadCloser, error) {
	logrus.Info("Downloading object from bucket ", s.bucket, ": ", location)
	return s.client.GetObject(s.bucket, location, minio.GetObjectOptions{})
}

func (s *s3Datastore) ObjectExists(location string) bool {
	stat, err := s.client.StatObject(s.bucket, location, minio.StatObjectOptions{})
	if err != nil {
		return false
	}
	return stat.Size > 0
}

func (s *s3Datastore) OverwriteObject(location string, stream io.ReadCloser) error {
	_, err := s.client.PutObject(s.bucket, location, stream, -1, minio.PutObjectOptions{})
	return err
}
