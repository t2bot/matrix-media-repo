package api

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

func NotFoundHandler(r *http.Request, log *logrus.Entry) interface{} {
	return NotFoundError()
}

func MethodNotAllowedHandler(r *http.Request, log *logrus.Entry) interface{} {
	return MethodNotAllowed()
}
