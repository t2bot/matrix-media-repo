package _routers

import (
	"context"
	"net/http"
	"strconv"
	"time"

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

func (router *InstallMetadataRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestId := router.counter.NextId()
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
	ctx = context.WithValue(ctx, common.ContextRequestStartTime, time.Now())
	ctx = context.WithValue(ctx, common.ContextRequestId, requestId)
	ctx = context.WithValue(ctx, common.ContextAction, router.actionName)
	ctx = context.WithValue(ctx, common.ContextIgnoreHost, router.ignoreHost)
	ctx = context.WithValue(ctx, common.ContextLogger, logger)
	r = r.WithContext(ctx)

	if router.next != nil {
		router.next.ServeHTTP(w, r)
	}
}

func GetActionName(r *http.Request) string {
	action, ok := r.Context().Value(common.ContextAction).(string)
	if !ok {
		return "<UNKNOWN>"
	}
	return action
}

func ShouldIgnoreHost(r *http.Request) bool {
	ignoreHost, ok := r.Context().Value(common.ContextIgnoreHost).(bool)
	if !ok {
		return false
	}
	return ignoreHost
}

func GetLogger(r *http.Request) *logrus.Entry {
	log, ok := r.Context().Value(common.ContextLogger).(*logrus.Entry)
	if !ok {
		return nil
	}
	return log
}

func GetRequestDuration(r *http.Request) float64 {
	duration, ok := r.Context().Value(common.ContextRequestStartTime).(time.Time)
	if !ok {
		return -1
	}
	return float64(time.Since(duration).Milliseconds())
}
