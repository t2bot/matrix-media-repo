package ds_s3

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v6"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

var stores = make(map[string]*s3Datastore)

type s3Datastore struct {
	conf     config.DatastoreConfig
	dsId     string
	client   *minio.Client
	bucket   string
	region   string
	tempPath string
}

func GetOrCreateS3Datastore(dsId string, conf config.DatastoreConfig) (*s3Datastore, error) {
	if s, ok := stores[dsId]; ok {
		return s, nil
	}

	endpoint, epFound := conf.Options["endpoint"]
	bucket, bucketFound := conf.Options["bucketName"]
	accessKeyId, keyFound := conf.Options["accessKeyId"]
	accessSecret, secretFound := conf.Options["accessSecret"]
	region, regionFound := conf.Options["region"]
	tempPath, tempPathFound := conf.Options["tempPath"]
	if !epFound || !bucketFound || !keyFound || !secretFound {
		return nil, errors.New("invalid configuration: missing s3 options")
	}
	if !tempPathFound {
		logrus.Warn("Datastore ", dsId, " (s3) does not have a tempPath set - this could lead to excessive memory usage by the media repo")
	}

	useSsl := true
	useSslStr, sslFound := conf.Options["ssl"]
	if sslFound && useSslStr != "" {
		useSsl, _ = strconv.ParseBool(useSslStr)
	}

	var s3client *minio.Client
	var err error

	if regionFound {
		s3client, err = minio.NewWithRegion(endpoint, accessKeyId, accessSecret, useSsl, region)
	} else {
		s3client, err = minio.New(endpoint, accessKeyId, accessSecret, useSsl)
	}
	if err != nil {
		return nil, err
	}

	s3ds := &s3Datastore{
		conf:     conf,
		dsId:     dsId,
		client:   s3client,
		bucket:   bucket,
		region:   region,
		tempPath: tempPath,
	}
	stores[dsId] = s3ds
	return s3ds, nil
}

func GetS3URL(datastoreId string, location string) (string, error) {
	var store *s3Datastore
	var ok bool
	if store, ok = stores[datastoreId]; !ok {
		return "", errors.New("s3 datastore not found")
	}

	// HACK: Surely there's a better way...
	return fmt.Sprintf("https://%s/%s/%s", store.conf.Options["endpoint"], store.bucket, location), nil
}

func ParseS3URL(s3url string) (string, string, string, error) {
	trimmed := s3url[8:] // trim off https
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", "", "", errors.New("invalid url")
	}

	endpoint := parts[0]
	location := parts[len(parts)-1]
	bucket := strings.Join(parts[1:len(parts)-1], "/")

	return endpoint, bucket, location, nil
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

func (s *s3Datastore) EnsureTempPathExists() error {
	err := os.MkdirAll(s.tempPath, os.ModePerm)
	if err != os.ErrExist && err != nil {
		return err
	}
	return nil
}

func (s *s3Datastore) UploadFile(file io.ReadCloser, expectedLength int64, ctx rcontext.RequestContext) (*types.ObjectInfo, error) {
	defer cleanup.DumpAndCloseStream(file)

	objectName, err := util.GenerateRandomString(512)
	if err != nil {
		return nil, err
	}

	var rs3 io.ReadCloser
	var ws3 io.WriteCloser
	rs3, ws3 = io.Pipe()
	tr := io.TeeReader(file, ws3)

	done := make(chan bool)
	defer close(done)

	var hash string
	var sizeBytes int64
	var hashErr error
	var uploadErr error

	go func() {
		defer ws3.Close()
		ctx.Log.Info("Calculating hash of stream...")
		hash, hashErr = util.GetSha256HashOfStream(ioutil.NopCloser(tr))
		ctx.Log.Info("Hash of file is ", hash)
		done <- true
	}()

	go func() {
		if expectedLength <= 0 {
			if s.tempPath != "" {
				ctx.Log.Info("Buffering file to temp path due to unknown file size")
				var f *os.File
				f, uploadErr = ioutil.TempFile(s.tempPath, "mr*")
				if uploadErr != nil {
					io.Copy(ioutil.Discard, rs3)
					done <- true
					return
				}
				defer os.Remove(f.Name())
				expectedLength, uploadErr = io.Copy(f, rs3)
				cleanup.DumpAndCloseStream(f)
				f, uploadErr = os.Open(f.Name())
				if uploadErr != nil {
					done <- true
					return
				}
				rs3 = f
				defer cleanup.DumpAndCloseStream(f)
			} else {
				ctx.Log.Warn("Uploading content of unknown length to s3 - this could result in high memory usage")
				expectedLength = -1
			}
		}
		ctx.Log.Info("Uploading file...")
		sizeBytes, uploadErr = s.client.PutObjectWithContext(ctx, s.bucket, objectName, rs3, expectedLength, minio.PutObjectOptions{})
		ctx.Log.Info("Uploaded ", sizeBytes, " bytes to s3")
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
	defer cleanup.DumpAndCloseStream(stream)
	_, err := s.client.PutObject(s.bucket, location, stream, -1, minio.PutObjectOptions{})
	return err
}
