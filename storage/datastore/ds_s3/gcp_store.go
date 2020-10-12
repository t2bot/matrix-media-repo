package ds_s3

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
	"google.golang.org/api/option"
)

var storesGcp = make(map[string]*gcpDatastore)

type gcpDatastore struct {
	conf     config.DatastoreConfig
	dsId     string
	client   *storage.Client
	bucket   string
	tempPath string
	ctx      context.Context
}

func GetOrCreateGCPDatastore(dsId string, conf config.DatastoreConfig) (*gcpDatastore, error) {
	if s, ok := storesGcp[dsId]; ok {
		return s, nil
	}

	bucket, bucketFound := conf.Options["bucketName"]
	tempPath, tempPathFound := conf.Options["tempPath"]
	jsonPath, jsonPathFound := conf.Options["jsonPath"]
	if !jsonPathFound || !bucketFound {
		return nil, errors.New("can't find gcp optinos")
	}
	if !tempPathFound {
		logrus.Warn("Datastore ", dsId, " (gcp) does not have a tempPath set "+
			"- this could lead to excessive memory usage by the media repo")
	}

	//useSsl := true
	//useSslStr, sslFound := conf.Options["ssl"]
	//if sslFound && useSslStr != "" {
	//	useSsl, _ = strconv.ParseBool(useSslStr)
	//}

	ctx := context.Background()

	gcpclient, err := storage.NewClient(ctx, option.WithCredentialsFile(jsonPath))

	if err != nil {
		return nil, err
	}

	gcpds := &gcpDatastore{
		conf:     conf,
		dsId:     dsId,
		client:   gcpclient,
		bucket:   bucket,
		tempPath: tempPath,
		ctx:      ctx,
	}
	storesGcp[dsId] = gcpds
	return gcpds, nil
}

func (s *gcpDatastore) EnsureBucketExists() error {
	_, err := s.client.Bucket(s.bucket).Attrs(s.ctx)
	if err == storage.ErrBucketNotExist {
		return errors.New("bucket not found")
	}
	return nil
}

func (s *gcpDatastore) EnsureTempPathExists() error {
	err := os.MkdirAll(s.tempPath, os.ModePerm)
	if err != os.ErrExist && err != nil {
		return err
	}
	return nil
}

func (s *gcpDatastore) UploadFile(file io.ReadCloser, expectedLength int64, ctx rcontext.RequestContext) (*types.ObjectInfo, error) {
	defer cleanup.DumpAndCloseStream(file)

	objectName, err := util.GenerateRandomString(512)
	if err != nil {
		return nil, err
	}

	var rgcp io.ReadCloser
	var wgcp io.WriteCloser
	rgcp, wgcp = io.Pipe()
	tr := io.TeeReader(file, wgcp)

	done := make(chan bool)
	defer close(done)

	var hash string
	var sizeBytes int64
	var hashErr error
	var uploadErr error

	go func() {
		defer wgcp.Close()
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
					io.Copy(ioutil.Discard, rgcp)
					done <- true
					return
				}
				defer os.Remove(f.Name())
				expectedLength, uploadErr = io.Copy(f, rgcp)
				cleanup.DumpAndCloseStream(f)
				f, uploadErr = os.Open(f.Name())
				if uploadErr != nil {
					done <- true
					return
				}
				rgcp = f
				defer cleanup.DumpAndCloseStream(f)
			} else {
				ctx.Log.Warn("Uploading content of unknown length to s3 " +
					"- this could result in high memory usage")
				expectedLength = -1
			}
		}
		ctx.Log.Info("Uploading file...")
		//sizeBytes, uploadErr = s.client.PutObjectWithContext(ctx, s.bucket, objectName, rgcp, expectedLength, minio.PutObjectOptions{})
		wc := s.client.Bucket(s.bucket).Object(objectName).NewWriter(ctx)
		//var body[] byte
		//_, err = rgcp.Read(body)
		//if err != nil{
		//	ctx.Log.Info(uploadErr)
		//}
		//sizeInt, uploadErr := wc.Write(body)
		//if uploadErr != nil{
		//	ctx.Log.Info(uploadErr)
		//}
		//sizeBytes = int64(sizeInt)
		if sizeBytes, err = io.Copy(wc, rgcp); err != nil {
			fmt.Errorf("io.Copy: %v", err)
		}
		if err := wc.Close(); err != nil {
			fmt.Errorf("Writer.Close: %v", err)
		}
		ctx.Log.Info("Uploaded ", sizeBytes, " bytes to gcp")
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

func (s *gcpDatastore) DeleteObject(location string) error {
	logrus.Info("Deleting object from bucket ", s.bucket, ": ", location)
	return s.client.Bucket(s.bucket).Object(location).Delete(s.ctx)
}

func (s *gcpDatastore) DownloadObject(location string) (io.ReadCloser, error) {
	logrus.Info("Downloading object from bucket ", s.bucket, ": ", location)
	return s.client.Bucket(s.bucket).Object(location).NewReader(s.ctx)
}

func (s *gcpDatastore) ObjectExists(location string) bool {
	stat, err := s.client.Bucket(s.bucket).Object(location).Attrs(s.ctx)
	if err != nil {
		return false
	}
	return stat.Size > 0
}

func (s *gcpDatastore) OverwriteObject(location string, stream io.ReadCloser) error {
	defer cleanup.DumpAndCloseStream(stream)
	//A new object will be created unless an object with this name already exists.
	//Otherwise any previous object with the same name will be replaced
	wc := s.client.Bucket(s.bucket).Object(location).NewWriter(s.ctx)
	_, err := io.Copy(wc, stream)
	if err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}

	return err
}
