package _routers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/alioygur/is"
	"github.com/getsentry/sentry-go"
	"github.com/t2bot/gotd-contrib/http_range"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type GeneratorFn = func(r *http.Request, ctx rcontext.RequestContext) interface{}

type RContextRouter struct {
	generatorFn GeneratorFn
	next        http.Handler
}

func NewRContextRouter(generatorFn GeneratorFn, next http.Handler) *RContextRouter {
	return &RContextRouter{generatorFn: generatorFn, next: next}
}

func (c *RContextRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if c.next != nil {
			c.next.ServeHTTP(w, r)
		}
	}()

	log := GetLogger(r)
	rctx := rcontext.RequestContext{
		Context: r.Context(),
		Log:     log,
		Config:  *GetDomainConfig(r),
		Request: r,
	}

	var res interface{}
	res = c.generatorFn(r, rctx)
	if res == nil {
		res = &_responses.EmptyResponse{}
	}

	shouldCache := true
	wrappedRes, isNoCache := res.(*_responses.DoNotCacheResponse)
	if isNoCache {
		shouldCache = false
		res = wrappedRes.Payload
	}

	headers := w.Header()

	// Check for redirection early
	if redirect, isRedirect := res.(*_responses.RedirectResponse); isRedirect {
		log.Infof("Replying with result: %T <%s>", res, redirect.ToUrl)
		headers.Set("Location", redirect.ToUrl)
		r = writeStatusCode(w, r, http.StatusTemporaryRedirect)
		return // we're done here
	}

	// Check for HTML response and reply accordingly
	if htmlRes, isHtml := res.(*_responses.HtmlResponse); isHtml {
		log.Infof("Replying with result: %T <%d chars of html>", res, len(htmlRes.HTML))

		// Write out HTML here, now that we know it's happening
		if shouldCache {
			headers.Set("Cache-Control", "private, max-age=259200") // 3 days
		}
		headers.Set("Content-Type", "text/html; charset=UTF-8")

		// Clear the CSP because we're serving HTML
		headers.Set("Content-Security-Policy", "")
		headers.Set("X-Content-Security-Policy", "")

		r = writeStatusCode(w, r, http.StatusOK)
		if _, err := w.Write([]byte(htmlRes.HTML)); err != nil {
			panic(errors.New("error sending HtmlResponse: " + err.Error()))
		}
		return // don't continue
	}

	// Next try handling the response as a download, which might turn into an error
	proposedStatusCode := http.StatusOK
	var stream io.ReadCloser
	expectedBytes := int64(0)
	var contentType string
beforeParseDownload:
	log.Infof("Replying with result: %T %+v", res, res)
	if downloadRes, isDownload := res.(*_responses.DownloadResponse); isDownload {
		var ranges []http_range.Range
		var err error
		if downloadRes.SizeBytes > 0 {
			ranges, err = http_range.ParseRange(r.Header.Get("Range"), downloadRes.SizeBytes, rctx.Config.Downloads.DefaultRangeChunkSizeBytes)
			if errors.Is(err, http_range.ErrInvalid) {
				proposedStatusCode = http.StatusRequestedRangeNotSatisfiable
				res = _responses.BadRequest("invalid range header")
				goto beforeParseDownload // reprocess `res`
			} else if errors.Is(err, http_range.ErrNoOverlap) {
				proposedStatusCode = http.StatusRequestedRangeNotSatisfiable
				res = _responses.BadRequest("out of range")
				goto beforeParseDownload // reprocess `res`
			}
			if len(ranges) > 1 {
				proposedStatusCode = http.StatusRequestedRangeNotSatisfiable
				res = _responses.BadRequest("only 1 range is supported")
				goto beforeParseDownload // reprocess `res`
			}
		}

		contentType = downloadRes.ContentType
		expectedBytes = downloadRes.SizeBytes

		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if shouldCache {
			headers.Set("Cache-Control", "private, max-age=259200") // 3 days
		}

		if downloadRes.SizeBytes > 0 {
			headers.Set("Accept-Ranges", "bytes")
		}

		disposition := downloadRes.TargetDisposition
		if disposition == "" {
			disposition = "attachment"
		} else if disposition == "infer" {
			if util.CanInline(contentType) {
				disposition = "inline"
			} else {
				disposition = "attachment"
			}
		}
		fname := downloadRes.Filename
		if fname == "" {
			exts, err := mime.ExtensionsByType(contentType)
			if err != nil {
				exts = nil
				sentry.CaptureException(err)
				log.Warn("Unexpected error inferring file extension: ", err)
			}
			ext := ""
			if exts != nil && len(exts) > 0 {
				ext = exts[0]
			}
			fname = "file" + ext
		}
		if is.ASCII(fname) {
			headers.Set("Content-Disposition", disposition+"; filename="+url.QueryEscape(fname))
		} else {
			headers.Set("Content-Disposition", disposition+"; filename*=utf-8''"+url.QueryEscape(fname))
		}

		stream = downloadRes.Data
		if len(ranges) > 0 {
			if rsc, ok := stream.(io.ReadSeekCloser); ok {
				target := ranges[0] // we only use the first range (validated up above)
				if _, err = rsc.Seek(target.Start, io.SeekStart); err != nil {
					rctx.Log.Warn("Non-fatal error seeking for Range request: ", err)
					sentry.CaptureException(err)
				} else {
					headers.Set("Content-Range", target.ContentRange(downloadRes.SizeBytes))
					proposedStatusCode = http.StatusPartialContent
					stream = readers.NewCancelCloser(io.NopCloser(io.LimitReader(rsc, target.Length)), func() {
						_ = rsc.Close()
					})
					expectedBytes = target.Length
				}
			}
		}
	}

	// Try to find a suitable error code, if one is needed
	if errRes, isError := res.(_responses.ErrorResponse); isError {
		res = &errRes // just fix it
	}
	if errRes, isError := res.(*_responses.ErrorResponse); isError && proposedStatusCode == http.StatusOK {
		switch errRes.InternalCode {
		case common.ErrCodeMissingToken:
			proposedStatusCode = http.StatusUnauthorized
			break
		case common.ErrCodeUnknownToken:
			proposedStatusCode = http.StatusUnauthorized
			break
		case common.ErrCodeNotFound:
			proposedStatusCode = http.StatusNotFound
			break
		case common.ErrCodeMediaTooLarge:
			proposedStatusCode = http.StatusRequestEntityTooLarge
			break
		case common.ErrCodeBadRequest:
			proposedStatusCode = http.StatusBadRequest
			break
		case common.ErrCodeMethodNotAllowed:
			proposedStatusCode = http.StatusMethodNotAllowed
			break
		case common.ErrCodeForbidden:
			proposedStatusCode = http.StatusForbidden
			break
		case common.ErrCodeNoGuests:
			proposedStatusCode = http.StatusForbidden
			break
		case common.ErrCodeCannotOverwrite:
			proposedStatusCode = http.StatusConflict
			break
		case common.ErrCodeNotYetUploaded:
			proposedStatusCode = http.StatusGatewayTimeout
			break
		default: // Treat as unknown (a generic server error)
			proposedStatusCode = http.StatusInternalServerError
			break
		}
	}

	// Prepare a stream if one isn't set, and assume JSON
	if stream == nil {
		contentType = "application/json"
		b, err := json.Marshal(res)
		if err != nil {
			panic(err) // blow up this request
		}
		stream = io.NopCloser(bytes.NewReader(b))
		expectedBytes = int64(len(b))
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		sentry.CaptureException(err)
		log.Warn("Failed to parse content type header for media on reply: ", err)
	} else {
		// TODO: Maybe we only strip the charset from images? Is it valid to have the param on other types?
		if !strings.HasPrefix(mediaType, "text/") && mediaType != "application/json" {
			delete(params, "charset")
		}
		contentType = mime.FormatMediaType(mediaType, params)
	}
	headers.Set("Content-Type", contentType)

	if expectedBytes > 0 {
		headers.Set("Content-Length", strconv.FormatInt(expectedBytes, 10))
	}

	r = writeStatusCode(w, r, proposedStatusCode)

	defer stream.Close()
	written, err := io.Copy(w, stream)
	if err != nil {
		panic(err) // blow up this request
	}
	if expectedBytes > 0 && written != expectedBytes {
		panic(errors.New(fmt.Sprintf("mismatch transfer size: %d expected, %d sent", expectedBytes, written)))
	}
}

func GetStatusCode(r *http.Request) int {
	x, ok := r.Context().Value(common.ContextStatusCode).(int)
	if !ok {
		return http.StatusOK
	}
	return x
}

func writeStatusCode(w http.ResponseWriter, r *http.Request, statusCode int) *http.Request {
	w.WriteHeader(statusCode)
	return r.WithContext(context.WithValue(r.Context(), common.ContextStatusCode, statusCode))
}
