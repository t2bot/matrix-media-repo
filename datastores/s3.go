package datastores

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

var s3clients = &sync.Map{}

type s3 struct {
	client             *minio.Client
	storageClass       string
	bucket             string
	publicBaseUrl      string
	redirectWhenCached bool
	prefixLength       int
	multipartUploads   bool
}

func ResetS3Clients() {
	s3clients = &sync.Map{}
}

func getS3(ds config.DatastoreConfig) (*s3, error) {
	if val, ok := s3clients.Load(ds.Id); ok {
		return val.(*s3), nil
	}

	endpoint := ds.Options["endpoint"]
	bucket := ds.Options["bucketName"]
	accessKeyId := ds.Options["accessKeyId"]
	accessSecret := ds.Options["accessSecret"]
	region := ds.Options["region"]
	storageClass, hasStorageClass := ds.Options["storageClass"]
	useSslStr, hasSsl := ds.Options["ssl"]
	publicBaseUrl := ds.Options["publicBaseUrl"]
	redirectWhenCachedStr, hasRedirectWhenCached := ds.Options["redirectWhenCached"]
	prefixLengthStr, hasPrefixLength := ds.Options["prefixLength"]
	useMultipartStr, hasMultipart := ds.Options["multipartUploads"]

	if !hasStorageClass {
		storageClass = "STANDARD"
	}

	useSsl := true
	if hasSsl && useSslStr != "" {
		useSsl, _ = strconv.ParseBool(useSslStr)
	}

	useMultipart := true
	if hasMultipart && useMultipartStr != "" {
		useMultipart, _ = strconv.ParseBool(useMultipartStr)
	}

	redirectWhenCached := false
	if hasRedirectWhenCached && redirectWhenCachedStr != "" {
		redirectWhenCached, _ = strconv.ParseBool(redirectWhenCachedStr)
	}

	prefixLength := 0
	if hasPrefixLength && prefixLengthStr != "" {
		prefixLength, _ = strconv.Atoi(prefixLengthStr)
		if prefixLength < 0 {
			prefixLength = 0
		}
		if prefixLength > 16 {
			logrus.Warnf("Prefix length %d is greater than 16 for datastore %s - this may cause future incompatibilities", prefixLength, ds.Id)
		}
	}

	var err error
	var client *minio.Client
	client, err = minio.New(endpoint, &minio.Options{
		Region: region,
		Secure: useSsl,
		Creds:  credentials.NewStaticV4(accessKeyId, accessSecret, ""),
	})
	if err != nil {
		return nil, err
	}

	s3c := &s3{
		client:             client,
		storageClass:       storageClass,
		bucket:             bucket,
		publicBaseUrl:      publicBaseUrl,
		redirectWhenCached: redirectWhenCached,
		prefixLength:       prefixLength,
		multipartUploads:   useMultipart,
	}
	s3clients.Store(ds.Id, s3c)
	return s3c, nil
}

func ListS3Files(ctx rcontext.RequestContext, ds config.DatastoreConfig) (<-chan minio.ObjectInfo, error) {
	if ds.Type != "s3" {
		return nil, errors.New("not an S3 datastore")
	}
	s3c, err := getS3(ds)
	if err != nil {
		return nil, err
	}
	return s3c.client.ListObjects(ctx.Context, s3c.bucket, minio.ListObjectsOptions{
		Recursive: false,
	}), nil
}

func GetS3Url(ds config.DatastoreConfig, location string) (string, error) {
	if ds.Type != "s3" {
		return "", nil
	}

	s3c, err := getS3(ds)
	if err != nil {
		return "", err
	}

	// HACK: Surely there's a better way...
	return fmt.Sprintf("%s/%s/%s", s3c.client.EndpointURL(), s3c.bucket, location), nil
}

func ParseS3Url(s3url string) (config.DatastoreConfig, string, error) {
	parts := strings.Split(s3url[len("https://"):], "/")
	if len(parts) < 3 {
		return config.DatastoreConfig{}, "", errors.New("invalid url")
	}

	endpoint := parts[0]
	location := parts[len(parts)-1]
	bucket := strings.Join(parts[1:len(parts)-1], "/")

	for _, c := range config.Get().DataStores {
		if c.Type != "s3" {
			continue
		}

		s3c, err := getS3(c)
		if err != nil {
			return config.DatastoreConfig{}, "", err
		}

		if s3c.client.EndpointURL().Host == endpoint && s3c.bucket == bucket {
			return c, location, nil
		}
	}

	return config.DatastoreConfig{}, "", errors.New("could not locate datastore")
}
