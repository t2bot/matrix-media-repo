package client

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

func NotFoundHandler(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	return NotFoundError()
}

func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	return MethodNotAllowed()
}
