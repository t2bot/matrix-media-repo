package _routers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/alioygur/is"
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
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
		doRange, rangeStart, rangeEnd, rangeErrMsg := parseRange(r, downloadRes)
		if doRange && rangeErrMsg != "" {
			proposedStatusCode = http.StatusRequestedRangeNotSatisfiable
			res = _responses.BadRequest(rangeErrMsg)
			doRange = false
			goto beforeParseDownload // reprocess `res`
		}

		contentType = downloadRes.ContentType
		expectedBytes = downloadRes.SizeBytes

		if shouldCache {
			headers.Set("Cache-Control", "private, max-age=259200") // 3 days
		}

		if downloadRes.SizeBytes > 0 {
			headers.Set("Accept-Ranges", "bytes")
		}

		disposition := downloadRes.TargetDisposition
		if disposition == "" {
			disposition = "inline"
		} else if disposition == "infer" {
			if contentType == "" {
				disposition = "attachment"
			} else {
				if util.CanInline(contentType) {
					disposition = "inline"
				} else {
					disposition = "attachment"
				}
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

		if _, ok := stream.(io.ReadSeekCloser); ok && doRange {
			headers.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, downloadRes.SizeBytes))
			proposedStatusCode = http.StatusPartialContent
		}
		stream = downloadRes.Data
	}

	// Try to find a suitable error code, if one is needed
	if errRes, isError := res.(_responses.ErrorResponse); isError {
		res = &errRes // just fix it
	}
	if errRes, isError := res.(*_responses.ErrorResponse); isError && proposedStatusCode == http.StatusOK {
		switch errRes.InternalCode {
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

	if c.next != nil {
		c.next.ServeHTTP(w, r)
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

func parseRange(r *http.Request, res *_responses.DownloadResponse) (bool, int64, int64, string) {
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" || res.SizeBytes <= 0 {
		return false, 0, 0, ""
	}

	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return true, 0, 0, "Improper range units"
	}
	if !strings.Contains(rangeHeader, ",") && !strings.HasPrefix(rangeHeader, "bytes=-") {
		parts := strings.Split(rangeHeader[len("bytes="):], "-")
		if len(parts) <= 2 {
			rstart, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return true, 0, 0, "Improper start of range"
			}
			if rstart < 0 {
				return true, 0, 0, "Improper start of range: negative"
			}

			rend := int64(-1)
			if len(parts) > 1 && parts[1] != "" {
				rend, err = strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					return true, 0, 0, "Improper end of range"
				}
				if rend < 1 {
					return true, 0, 0, "Improper end of range: negative"
				}
				if rend >= res.SizeBytes {
					return true, 0, 0, "Improper end of range: out of bounds"
				}
				if rend <= rstart {
					return true, 0, 0, "Start must be before end"
				}
				if (rstart + rend) >= res.SizeBytes {
					return true, 0, 0, "Range too large"
				}
			} else {
				add := int64(10485760) // 10mb default
				conf := GetDomainConfig(r)
				if conf.Downloads.DefaultRangeChunkSizeBytes > 0 {
					add = conf.Downloads.DefaultRangeChunkSizeBytes
				}
				rend = int64(math.Min(float64(rstart+add), float64(res.SizeBytes-1)))
			}

			if (rend - rstart) <= 0 {
				return true, 0, 0, "Range invalid at last pass"
			}
			return true, rstart, rend, ""
		}
	}
	return false, 0, 0, ""
}
