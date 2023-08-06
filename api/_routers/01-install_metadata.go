package _routers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/util"
)

const requestIdCtxKey = "mmr.request_id"
const actionNameCtxKey = "mmr.action"
const shouldIgnoreHostCtxKey = "mmr.ignore_host"
const loggerCtxKey = "mmr.logger"

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
	ctx = context.WithValue(ctx, requestIdCtxKey, requestId)
	ctx = context.WithValue(ctx, actionNameCtxKey, i.actionName)
	ctx = context.WithValue(ctx, shouldIgnoreHostCtxKey, i.ignoreHost)
	ctx = context.WithValue(ctx, loggerCtxKey, logger)
	r = r.WithContext(ctx)

	if i.next != nil {
		i.next.ServeHTTP(w, r)
	}
}

func GetActionName(r *http.Request) string {
	x, ok := r.Context().Value(actionNameCtxKey).(string)
	if !ok {
		return "<UNKNOWN>"
	}
	return x
}

func ShouldIgnoreHost(r *http.Request) bool {
	x, ok := r.Context().Value(shouldIgnoreHostCtxKey).(bool)
	if !ok {
		return false
	}
	return x
}

func GetLogger(r *http.Request) *logrus.Entry {
	x, ok := r.Context().Value(loggerCtxKey).(*logrus.Entry)
	if !ok {
		return nil
	}
	return x
}
