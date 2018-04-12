package webserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/sebest/xff"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/util"
)

const unkErrJson = `{"code":"M_UNKNOWN","message":"Unexpected error processing response"}`

type handler struct {
	h          func(r *http.Request, entry *logrus.Entry) interface{}
	reqCounter *requestCounter
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	isUsingForwardedHost := false
	if r.Header.Get("X-Forwarded-Host") != "" {
		r.Host = r.Header.Get("X-Forwarded-Host")
		isUsingForwardedHost = true
	}
	r.Host = strings.Split(r.Host, ":")[0]

	raddr := xff.GetRemoteAddr(r)
	host, _, err := net.SplitHostPort(raddr)
	if err != nil {
		logrus.Error(err)
		host = raddr
	}
	r.RemoteAddr = host

	contextLog := logrus.WithFields(logrus.Fields{
		"method":             r.Method,
		"host":               r.Host,
		"usingForwardedHost": isUsingForwardedHost,
		"resource":           r.URL.Path,
		"contentType":        r.Header.Get("Content-Type"),
		"contentLength":      r.ContentLength,
		"queryString":        util.GetLogSafeQueryString(r),
		"requestId":          h.reqCounter.GetNextId(),
		"remoteAddr":         r.RemoteAddr,
	})
	contextLog.Info("Received request")

	// Send CORS and other basic headers
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; plugin-types application/pdf; style-src 'unsafe-inline'; object-src 'self'")
	w.Header().Set("Cache-Control", "public,max-age=86400,s-maxage=86400")
	w.Header().Set("Server", "matrix-media-repo")

	// Process response
	var res interface{} = api.AuthFailed()
	if util.IsServerOurs(r.Host) {
		contextLog.Info("Server is owned by us, processing request")
		res = h.h(r, contextLog)
		if res == nil {
			res = &api.EmptyResponse{}
		}
	}

	b, err := json.Marshal(res)
	jsonStr := unkErrJson
	if err == nil {
		jsonStr = string(b)
	}

	contextLog.Info("Replying with result: " + reflect.TypeOf(res).Elem().Name() + " " + jsonStr)

	statusCode := http.StatusOK
	switch result := res.(type) {
	case *api.ErrorResponse:
		switch result.InternalCode {
		case "M_UNKNOWN_TOKEN":
			statusCode = http.StatusForbidden
			break
		case "M_NOT_FOUND":
			statusCode = http.StatusNotFound
			break
		case "M_MEDIA_TOO_LARGE":
			statusCode = http.StatusRequestEntityTooLarge
			break
		case "M_BAD_REQUEST":
			statusCode = http.StatusBadRequest
			break
		case "M_METHOD_NOT_ALLOWED":
			statusCode = http.StatusMethodNotAllowed
			break
		default: // M_UNKNOWN
			statusCode = http.StatusInternalServerError
			break
		}
		break
	case *r0.DownloadMediaResponse:
		w.Header().Set("Content-Type", result.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename=\""+result.Filename+"\"")
		w.Header().Set("Content-Length", fmt.Sprint(result.SizeBytes))
		defer result.Data.Close()
		io.Copy(w, result.Data)
		return // Prevent sending conflicting responses
	case *r0.IdenticonResponse:
		w.Header().Set("Content-Type", "image/png")
		io.Copy(w, result.Avatar)
		return // Prevent sending conflicting responses
	default:
		break
	}

	// Order is important: Set headers before sending responses
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	io.WriteString(w, jsonStr)
}
