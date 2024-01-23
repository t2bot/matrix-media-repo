package _routers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/util"
)

type RequestCounter struct {
	lastId uint64
}

func (c *RequestCounter) NextId() string {
	strId := strconv.FormatUint(c.lastId, 10)
	c.lastId = c.lastId + 1

	return "REQ-" + strId
}

type InstallMetadataRouter struct {
	next       http.Handler
	ignoreHost bool
	actionName string
	counter    *RequestCounter
}

func NewInstallMetadataRouter(ignoreHost bool, actionName string, counter *RequestCounter, next http.Handler) *InstallMetadataRouter {
	return &InstallMetadataRouter{
		next:       next,
		ignoreHost: ignoreHost,
		actionName: actionName,
		counter:    counter,
	}
}

func (i *InstallMetadataRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestId := i.counter.NextId()
	logger := logrus.WithFields(logrus.Fields{
		"method":        r.Method,
		"host":          r.Host,
		"resource":      r.URL.Path,
		"contentType":   r.Header.Get("Content-Type"),
		"contentLength": r.ContentLength,
		"queryString":   util.GetLogSafeQueryString(r),
		"requestId":     requestId,
		"remoteAddr":    r.RemoteAddr,
		"userAgent":     r.UserAgent(),
	})

	ctx := r.Context()
	ctx = context.WithValue(ctx, common.ContextRequestStartTime, util.NowMillis())
	ctx = context.WithValue(ctx, common.ContextRequestId, requestId)
	ctx = context.WithValue(ctx, common.ContextAction, i.actionName)
	ctx = context.WithValue(ctx, common.ContextIgnoreHost, i.ignoreHost)
	ctx = context.WithValue(ctx, common.ContextLogger, logger)
	r = r.WithContext(ctx)

	if i.next != nil {
		i.next.ServeHTTP(w, r)
	}
}

func GetActionName(r *http.Request) string {
	x, ok := r.Context().Value(common.ContextAction).(string)
	if !ok {
		return "<UNKNOWN>"
	}
	return x
}

func ShouldIgnoreHost(r *http.Request) bool {
	x, ok := r.Context().Value(common.ContextIgnoreHost).(bool)
	if !ok {
		return false
	}
	return x
}

func GetLogger(r *http.Request) *logrus.Entry {
	x, ok := r.Context().Value(common.ContextLogger).(*logrus.Entry)
	if !ok {
		return nil
	}
	return x
}

func GetRequestDuration(r *http.Request) float64 {
	x, ok := r.Context().Value(common.ContextRequestStartTime).(int64)
	if !ok {
		return -1
	}
	return float64(util.NowMillis()-x) / 1000.0
}
