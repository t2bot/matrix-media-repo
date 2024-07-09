package download

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/errcache"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/datastore_op"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type downloadResult struct {
	r           io.ReadCloser
	metadata    *database.AnonymousJson
	filename    string
	contentType string
	err         error
}

func TryDownload(ctx rcontext.RequestContext, origin string, mediaId string) (*database.DbMedia, io.ReadCloser, error) {
	if util.IsServerOurs(origin) {
		return nil, nil, common.ErrMediaNotFound
	}

	ch := make(chan downloadResult)
	defer close(ch)
	fn := func() {
		cacheKey := fmt.Sprintf("%s/%s", origin, mediaId)
		if err := errcache.DownloadErrors.Get(cacheKey); err != nil {
			ch <- downloadResult{err: err}
			return
		}

		errFn := func(err error) {
			errcache.DownloadErrors.Set(cacheKey, err)
			ch <- downloadResult{err: err}
		}

		baseUrl, realHost, err := matrix.GetServerApiUrl(origin)
		if err != nil {
			errFn(err)
			return
		}

		var resp *http.Response
		var downloadUrl string
		usesMultipartFormat := false
		if ctx.Config.SigningKeyPath != "" {
			downloadUrl = fmt.Sprintf("%s/_matrix/federation/v1/media/download/%s", baseUrl, url.PathEscape(mediaId))
			resp, err = matrix.FederatedGet(ctx, downloadUrl, realHost, origin, ctx.Config.SigningKeyPath)
			metrics.MediaDownloaded.With(prometheus.Labels{"origin": origin}).Inc()
			if err != nil {
				errFn(err)
				return
			}
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
				errFn(matrix.MakeServerNotAllowedError(ctx.Request.Host))
				return
			} else if resp.StatusCode == http.StatusNotFound {
				decoder := json.NewDecoder(resp.Body)
				resp2 := resp // copy response in case we clear it out later
				defer resp2.Body.Close()
				mxerr := &matrix.ErrorResponse{}
				if err = decoder.Decode(&mxerr); err != nil {
					// we probably got not-json - ignore and move on
					ctx.Log.Debugf("Ignoring JSON decoding error on download error %d: %v", resp.StatusCode, err)
					resp = nil // indicate we want to use fallback
				} else {
					if mxerr.ErrorCode == "M_UNRECOGNIZED" {
						ctx.Log.Debugf("Destination doesn't support MSC3916")
						resp = nil // indicate we want to use fallback
					}
				}
			} else if resp.StatusCode == http.StatusOK {
				usesMultipartFormat = true
			}
		} else {
			// Yes, we are deliberately loud about this. People should configure this.
			ctx.Log.Warn("No signing key is configured for this domain! See `signingKeyPath` in the sample config for details.")
		}

		// Try fallback (unauthenticated)
		if resp == nil {
			downloadUrl = fmt.Sprintf("%s/_matrix/media/v3/download/%s/%s?allow_remote=false&allow_redirect=true", baseUrl, url.PathEscape(origin), url.PathEscape(mediaId))
			resp, err = matrix.FederatedGet(ctx, downloadUrl, realHost, origin, matrix.NoSigningKey)
			metrics.MediaDownloaded.With(prometheus.Labels{"origin": origin}).Inc()
			if err != nil {
				errFn(err)
				return
			}
		}

		if resp.StatusCode == http.StatusNotFound {
			errFn(common.ErrMediaNotFound)
			return
		} else if resp.StatusCode != http.StatusOK {
			errFn(errors.New(fmt.Sprintf("unexpected status code %d", resp.StatusCode)))
			return
		}

		contentLength := int64(0)
		if resp.Header.Get("Content-Length") != "" {
			contentLength, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
			if err != nil {
				errFn(err)
				return
			}
		}

		if contentLength != 0 && ctx.Config.Downloads.MaxSizeBytes > 0 && contentLength > ctx.Config.Downloads.MaxSizeBytes {
			errFn(common.ErrMediaTooLarge)
			return
		}

		if ctx.Config.Downloads.MaxSizeBytes > 0 {
			resp.Body = readers.LimitReaderWithOverrunError(resp.Body, ctx.Config.Downloads.MaxSizeBytes)
		}

		contentType := resp.Header.Get("Content-Type") // we default Content-Type after we inspect for multiparts

		metadata := &database.AnonymousJson{}
		mediaPart := util.MatrixMediaPartFromResponse(resp)
		if usesMultipartFormat {
			if !strings.HasPrefix(contentType, "multipart/mixed;") {
				errFn(fmt.Errorf("expected multipart/mixed, got %s", contentType))
				return
			}

			_, params, err := mime.ParseMediaType(contentType)
			if err != nil {
				errFn(err)
				return
			}

			partReader := multipart.NewReader(resp.Body, params["boundary"])

			// The first part should always be the metadata
			jsonPart, err := partReader.NextPart()
			if err != nil {
				errFn(err)
				return
			}
			partType := jsonPart.Header.Get("Content-Type")
			if partType == "" || partType == "application/json" {
				decoder := json.NewDecoder(jsonPart)
				err = decoder.Decode(&metadata)
				if err != nil {
					errFn(err)
					return
				}
			} else {
				errFn(fmt.Errorf("expected application/json as the first part, got %s instead", partType))
			}

			ctx.Log.Debugf("Got metadata: %v", metadata)

			// The second part should always be the media itself
			bodyPart, err := partReader.NextPart()
			if err != nil {
				errFn(err)
				return
			}
			mediaPart = util.MatrixMediaPartFromMimeMultipart(bodyPart)
			contentType = mediaPart.Header.Get("Content-Type") // Content-Type should really be the media content type

			locationHeader := mediaPart.Header.Get("Location")
			if locationHeader != "" {
				// the media part body won't have anything for us - go `GET` the URL.
				ctx.Log.Debugf("Redirecting to %s", locationHeader)

				err = mediaPart.Body.Close()
				if err != nil {
					sentry.CaptureException(errors.Join(errors.New("non-fatal error closing redirected MSC3916 body"), err))
					ctx.Log.Debug("Non-fatal error closing redirected MSC3916 body: ", err)
				}

				resp, err = http.DefaultClient.Get(locationHeader)
				if err != nil {
					errFn(err)
					return
				}
				mediaPart = util.MatrixMediaPartFromResponse(resp)
				contentType = mediaPart.Header.Get("Content-Type")
			}
		}

		// Default the Content-Type if we haven't already
		if contentType == "" {
			contentType = "application/octet-stream" // binary
		}

		fileName := "download"
		_, params, err := mime.ParseMediaType(mediaPart.Header.Get("Content-Disposition"))
		if err == nil && params["filename"] != "" {
			fileName = params["filename"]
		}

		ch <- downloadResult{
			r:           mediaPart.Body,
			metadata:    metadata,
			filename:    fileName,
			contentType: contentType,
			err:         nil,
		}
	}
	if err := pool.DownloadQueue.Schedule(fn); err != nil {
		return nil, nil, err
	}
	res := <-ch
	if res.err != nil {
		return nil, nil, res.err
	}

	// At this point, res.r is our http response body.
	// TODO: Do something with res.metadata (MSC3911)

	return datastore_op.PutAndReturnStream(ctx, origin, mediaId, res.r, res.contentType, res.filename, datastores.RemoteMediaKind)
}
