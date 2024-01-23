package r0

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_preview"

	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
)

type MatrixOpenGraph struct {
	Url         string `json:"og:url,omitempty"`
	SiteName    string `json:"og:site_name,omitempty"`
	Type        string `json:"og:type,omitempty"`
	Description string `json:"og:description,omitempty"`
	Title       string `json:"og:title,omitempty"`
	ImageMxc    string `json:"og:image,omitempty"`
	ImageType   string `json:"og:image:type,omitempty"`
	ImageSize   int64  `json:"matrix:image:size,omitempty"`
	ImageWidth  int    `json:"og:image:width,omitempty"`
	ImageHeight int    `json:"og:image:height,omitempty"`
}

func PreviewUrl(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.UrlPreviews.Enabled {
		return _responses.NotFoundError()
	}

	params := r.URL.Query()

	// Parse the parameters
	urlStr := params.Get("url")
	tsStr := params.Get("ts")
	ts := util.NowMillis()
	var err error
	if tsStr != "" {
		ts, err = strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			rctx.Log.Error("Error parsing ts: ", err)
			return _responses.BadRequest(err.Error())
		}
	}

	// Validate the URL
	if urlStr == "" {
		return _responses.BadRequest("No url provided")
	}
	//goland:noinspection HttpUrlsUsage
	if strings.Index(urlStr, "http://") != 0 && strings.Index(urlStr, "https://") != 0 {
		return _responses.BadRequest("Scheme not accepted")
	}

	languageHeader := rctx.Config.UrlPreviews.DefaultLanguage
	if r.Header.Get("Accept-Language") != "" {
		languageHeader = r.Header.Get("Accept-Language")
	}

	preview, err := pipeline_preview.Execute(rctx, r.Host, urlStr, user.UserId, pipeline_preview.PreviewOpts{
		Timestamp:      ts,
		LanguageHeader: languageHeader,
	})
	if err == nil && preview != nil && preview.ErrorCode != "" {
		if preview.ErrorCode == common.ErrCodeInvalidHost {
			err = common.ErrInvalidHost
		} else if preview.ErrorCode == common.ErrCodeNotFound {
			err = common.ErrMediaNotFound
		} else {
			err = errors.New("url previews: unknown error code: " + preview.ErrorCode)
		}
	}
	if err != nil {
		if errors.Is(err, common.ErrMediaNotFound) || errors.Is(err, common.ErrHostNotFound) {
			return _responses.NotFoundError()
		} else if errors.Is(err, common.ErrInvalidHost) || errors.Is(err, common.ErrHostNotAllowed) {
			return _responses.BadRequest(err.Error())
		} else {
			sentry.CaptureException(err)
			return _responses.InternalServerError("Unexpected Error")
		}
	}

	return &MatrixOpenGraph{
		Url:         preview.SiteUrl,
		SiteName:    preview.SiteName,
		Type:        preview.ResourceType,
		Description: preview.Description,
		Title:       preview.Title,
		ImageMxc:    preview.ImageMxc,
		ImageType:   preview.ImageType,
		ImageSize:   preview.ImageSize,
		ImageWidth:  preview.ImageWidth,
		ImageHeight: preview.ImageHeight,
	}
}
