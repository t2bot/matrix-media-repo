package url_service

import (
	bytes2 "bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"

	"github.com/ryanuber/go-glob"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util"
)

type filePreviewer struct {
	ctx context.Context
	log *logrus.Entry
}

func NewFilePreviewer(ctx context.Context, log *logrus.Entry) *filePreviewer {
	return &filePreviewer{ctx, log}
}

func (p *filePreviewer) GeneratePreview(urlStr string) (previewResult, error) {
	img, err := downloadFileContent(urlStr, p.log)
	if err != nil {
		p.log.Error("Error downloading content: " + err.Error())

		// Make sure the unsupported error gets passed through
		if err == ErrPreviewUnsupported {
			return previewResult{}, ErrPreviewUnsupported
		}

		// We'll consider it not found for the sake of processing
		return previewResult{}, common.ErrMediaNotFound
	}

	description := ""
	filename := urlStr
	if img != nil && img.Filename != "" {
		filename = img.Filename
	} else {
		description = urlStr
	}

	// Clear the description so we don't duplicate the URL
	if description == filename {
		description = ""
	}

	result := &previewResult{
		Type:        "", // intentionally empty
		Url:         urlStr,
		Title:       summarize(filename, config.Get().UrlPreviews.NumTitleWords, config.Get().UrlPreviews.MaxTitleLength),
		Description: summarize(description, config.Get().UrlPreviews.NumWords, config.Get().UrlPreviews.MaxLength),
		SiteName:    "", // intentionally empty
	}

	if glob.Glob("image/*", img.ContentType) {
		result.Image = img
	}

	return *result, nil
}

func downloadFileContent(urlStr string, log *logrus.Entry) (*previewImage, error) {
	log.Info("Fetching remote content...")
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return nil, errors.New("error during transfer")
	}

	if config.Get().UrlPreviews.MaxPageSizeBytes > 0 && resp.ContentLength >= 0 && resp.ContentLength > config.Get().UrlPreviews.MaxPageSizeBytes {
		return nil, common.ErrMediaTooLarge
	}

	var reader io.Reader
	reader = resp.Body
	if config.Get().UrlPreviews.MaxPageSizeBytes > 0 {
		reader = io.LimitReader(resp.Body, config.Get().UrlPreviews.MaxPageSizeBytes)
	}

	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if len(config.Get().UrlPreviews.FilePreviewTypes) > 0 {
		for _, allowedType := range config.Get().UrlPreviews.FilePreviewTypes {
			if !glob.Glob(allowedType, contentType) {
				log.Warn("Content type " + contentType + " is not allowed and therefore not supported")
				return nil, ErrPreviewUnsupported
			}
		}
	}

	disposition := resp.Header.Get("Content-Disposition")
	_, params, _ := mime.ParseMediaType(disposition)
	filename := ""
	if params != nil {
		filename = params["filename"]
	}

	stream := util.BufferToStream(bytes2.NewBuffer(bytes))
	return &previewImage{
		Data:                stream,
		ContentType:         contentType,
		Filename:            filename,
		ContentLength:       int64(len(bytes)),
		ContentLengthHeader: resp.Header.Get("Content-Length"),
	}, nil
}
